package controller_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"ardents/internal/persistence"
	workloadcontroller "ardents/internal/workload/controller"

	"github.com/stretchr/testify/require"
)

func TestRegisterPresentWorkloadRemainsAccepted(t *testing.T) {
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), workloadcontroller.NewLocalExecutor())
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		err := svc.Register(workloadcontroller.Spec{ID: "work.echo", Kind: "service", Owner: "node", Desired: workloadcontroller.DesiredPresent})
		require.NoErrorf(t, err, "register: %v", err)
	}
	{

		err := svc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile: %v", err)
	}

	item, ok := svc.Get("work.echo")
	require.True(t, ok, "expected workload")
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedAccepted, "observed = %q, want accepted", item.Observed)
	require.False(t, item.Instance.Running, "did not expect running instance")

}

func TestUnsupportedKindFailsAdmission(t *testing.T) {
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), workloadcontroller.NewLocalExecutor())
	{
		err := svc.Register(workloadcontroller.Spec{ID: "work.bad", Kind: "unknown", Owner: "node", Desired: workloadcontroller.DesiredRunning})
		require.NoErrorf(t, err, "register: %v", err)
	}
	{

		err := svc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile: %v", err)
	}

	item, ok := svc.Get("work.bad")
	require.True(t, ok, "expected workload")
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedFailed, "observed = %q, want failed", item.Observed)
	require.True(t, item.NeedsOperatorAction, "expected operator action requirement")

}

func TestRepeatedStartFailureBecomesFailed(t *testing.T) {
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), failExecutor{})
	{
		err := svc.Register(workloadcontroller.Spec{ID: "work.fail", Kind: "service", Owner: "node", Desired: workloadcontroller.DesiredRunning})
		require.NoErrorf(t, err, "register: %v", err)
	}

	for i := 0; i < workloadcontroller.DefaultRestartBudget+1; i++ {
		{
			err := svc.Reconcile(context.Background())
			require.NoErrorf(t, err, "reconcile %d: %v", i, err)
		}

	}
	item, ok := svc.Get("work.fail")
	require.True(t, ok, "expected workload")
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedFailed, "observed = %q, want failed", item.Observed)
	require.Falsef(t, item.RestartCount <=
		workloadcontroller.
			DefaultRestartBudget, "restart count = %d, want > %d", item.RestartCount, workloadcontroller.DefaultRestartBudget)
	require.True(t, item.NeedsOperatorAction, "expected operator action requirement")

}

func TestInspectFailureDegradesWithoutRestart(t *testing.T) {
	executor := &inspectFailureExecutor{}
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), executor)
	{
		err := svc.Register(workloadcontroller.Spec{
			ID:      "work.inspect",
			Kind:    "service",
			Owner:   "node",
			Config:  helperProcessConfig(t, "sleep"),
			Desired: workloadcontroller.DesiredRunning,
			Services: []workloadcontroller.ServiceSpec{{
				ID:        "svc.inspect",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://inspect:9000"},
			}},
		})
		require.NoErrorf(t, err, "register: %v", err)
	}
	{

		err := svc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile start: %v", err)
	}

	executor.inspectErr = errors.New("temporary inspect failure")
	{
		err := svc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile inspect failure: %v", err)
	}

	item, ok := svc.Get("work.inspect")
	require.True(t, ok, "expected workload")
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedDegraded, "observed = %q, want degraded", item.Observed)
	require.Truef(t, strings.HasPrefix(item.
		Reason,
		"inspect failed",
	), "reason = %q, want inspect failed", item.Reason)
	require.True(t, item.NeedsOperatorAction, "expected operator action requirement")
	require.Falsef(t, executor.startCalls !=
		1, "start calls = %d, want 1", executor.startCalls)
	require.False(t, len(svc.Published()) !=
		0, "expected inspect failure to withdraw published services")

}

func TestRegisterRejectsDuplicateWorkloadID(t *testing.T) {
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), workloadcontroller.NewLocalExecutor())
	spec := workloadcontroller.Spec{ID: "work.echo", Kind: "service", Owner: "node", Desired: workloadcontroller.DesiredPresent}
	{
		err := svc.Register(spec)
		require.NoErrorf(t, err, "register: %v", err)
	}

	err := svc.Register(spec)
	require.Error(t, err, "expected duplicate register error")
	require.Truef(t, strings.Contains(err.
		Error(), "already exists",
	), "error = %v, want already exists", err)

	item, ok := svc.Get("work.echo")
	require.True(t, ok, "expected workload to remain present")
	require.Falsef(t, item.Spec.ID != "work.echo" ||
		item.Observed !=
			workloadcontroller.
				ObservedAccepted, "item = %#v", item)

}

func TestRefreshObservedMarksExitedRuntimeAsDegradedAndUnpublished(t *testing.T) {
	svc := workloadcontroller.New(filepath.Join(t.TempDir(), "ardents.db"), workloadcontroller.NewLocalExecutor())
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		err := svc.Register(workloadcontroller.Spec{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  helperProcessConfig(t, "sleep"),
			Desired: workloadcontroller.DesiredRunning,
			Services: []workloadcontroller.ServiceSpec{{
				ID:        "svc.echo",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://echo:9000"},
			}},
		})
		require.NoErrorf(t, err, "register: %v", err)
	}
	{

		err := svc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile: %v", err)
	}

	item, ok := svc.Get("work.echo")
	require.Falsef(t, !ok || !item.Instance.
		Running, "item=%#v, want running instance", item)
	{

		err := workloadcontroller.NewLocalExecutor().Stop(context.Background(), item.Instance)
		require.NoErrorf(t, err, "stop external process by pid: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		changed, err := svc.RefreshObserved(context.Background())
		require.NoErrorf(t, err, "refresh observed: %v", err)

		item, ok = svc.Get("work.echo")
		require.True(t, ok, "expected workload after refresh")

		if changed && item.Observed == workloadcontroller.ObservedDegraded {
			break
		}
		require.Falsef(t, time.Now().After(deadline), "observed=%q reason=%q, want degraded after process exit", item.Observed, item.Reason)

		time.Sleep(20 * time.Millisecond)
	}
	require.False(t, len(svc.Published()) !=
		0, "expected observed refresh to withdraw published services")

}

func TestLoadRejectsPidReuseWhenProcessDoesNotMatchConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ardents.db")
	cmd := helperCommand(t, "sleep")
	{
		err := cmd.Start()
		require.NoErrorf(t, err, "start helper process: %v", err)
	}

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	startedAt := time.Now().UTC()
	state := map[string]any{
		"items": map[string]any{
			"work.echo": map[string]any{
				"spec": map[string]any{
					"id":             "work.echo",
					"kind":           "service",
					"owner":          "node",
					"config":         `{"command":"definitely-not-the-helper","args":["--nope"]}`,
					"desired":        workloadcontroller.DesiredRunning,
					"restart_policy": workloadcontroller.DefaultRestartPolicy,
					"services": []map[string]any{{
						"id":        "svc.echo",
						"type":      "echo",
						"owner":     "work.echo",
						"mode":      "NetworkPublished",
						"published": true,
						"endpoints": []string{"tcp://echo:9000"},
						"reason":    "",
					}},
				},
				"observed": workloadcontroller.ObservedRunning,
				"instance": map[string]any{
					"workload_id": "work.echo",
					"generation":  time.Now().UTC().UnixNano(),
					"running":     true,
					"pid":         cmd.Process.Pid,
					"started_at":  startedAt,
				},
				"published_services": []map[string]any{{
					"id":        "svc.echo",
					"type":      "echo",
					"owner":     "work.echo",
					"mode":      "NetworkPublished",
					"published": true,
					"endpoints": []string{"tcp://echo:9000"},
				}},
			},
		},
	}
	{
		err := persistence.SaveJSON(filepath.Join(filepath.Dir(path), "ardents.db"), "workload", "snapshot", state)
		require.NoErrorf(t, err, "write state: %v", err)
	}

	svc := workloadcontroller.New(path, workloadcontroller.NewLocalExecutor())
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	item, ok := svc.Get("work.echo")
	require.True(t, ok, "expected workload after load")
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedDegraded, "observed=%q, want degraded", item.Observed)
	require.Falsef(t, item.Instance.Running, "instance=%#v, want stopped after config mismatch", item.Instance)
	require.False(t, len(svc.Published()) !=
		0, "expected config-mismatched pid to stay unpublished")

}

func helperCommand(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	switch mode {
	case "sleep":
		if goruntime.GOOS == "windows" {
			return exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
		}
		return exec.Command("sh", "-c", "sleep 30")
	default:
		require.FailNowf(t, "unsupported helper mode %q", mode)
		return nil
	}
}

type failExecutor struct{}

type inspectFailureExecutor struct {
	startCalls int
	inspectErr error
}

func (failExecutor) Prepare(context.Context, workloadcontroller.Spec) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{
		WorkloadID: "work.fail",
		Generation: time.Now().UTC().UnixNano(),
		PreparedAt: time.Now().UTC(),
	}, nil
}

func (failExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{}, fmt.Errorf("boom")
}

func (failExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	return nil
}

func (failExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{}, fmt.Errorf("not found")
}

func (e *inspectFailureExecutor) Prepare(context.Context, workloadcontroller.Spec) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{
		WorkloadID: "work.inspect",
		Generation: time.Now().UTC().UnixNano(),
		PreparedAt: time.Now().UTC(),
	}, nil
}

func (e *inspectFailureExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	e.startCalls++
	return workloadcontroller.Instance{
		WorkloadID: "work.inspect",
		Generation: time.Now().UTC().UnixNano(),
		Running:    true,
		StartedAt:  time.Now().UTC(),
	}, nil
}

func (e *inspectFailureExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	return nil
}

func (e *inspectFailureExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	if e.inspectErr != nil {
		return workloadcontroller.Instance{}, e.inspectErr
	}
	return workloadcontroller.Instance{
		WorkloadID: "work.inspect",
		Running:    true,
		StartedAt:  time.Now().UTC(),
	}, nil
}

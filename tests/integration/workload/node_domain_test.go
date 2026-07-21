//go:build integration

package workload_test

import (
	workloadregistry "ardents/internal/workload/registry"
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	runtimeinfra "ardents/internal/daemon"
	"ardents/internal/diagnostics"
	transport "ardents/internal/network"
	runtimepublication "ardents/internal/publication"
	workloadapi "ardents/internal/workload"
	workloadcontroller "ardents/internal/workload/execution"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestWorkloadNodeStopClearsRestartRecoveryMarker(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)

	dir := t.TempDir()
	first := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "work-stop",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.echo",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"quic://echo:9000"},
			}},
		}},
	})
	{
		err := first.Node.Start(context.Background())
		require.NoErrorf(t, err, "start first node: %v", err)
	}

	status, ok := testkit.WorkloadStatusForIntegrationTest(first, "work.echo")
	require.Falsef(t, !ok || status.Instance.
		PID ==
		0, "status = %#v, want running workload pid", status)
	require.Truef(t, testkit.ProcessRunning(
		t, status.Instance.PID,
	), "expected workload pid %d to be alive before node stop", status.Instance.PID)
	{

		err := first.Node.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}
	require.Falsef(t, testkit.ProcessRunning(
		t, status.Instance.PID,
	), "expected workload pid %d to exit during node stop", status.Instance.PID)

	second := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "work-stop",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.echo",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"quic://echo:9000"},
			}},
		}},
	})
	{
		err := second.Node.Start(context.Background())
		require.NoErrorf(t, err, "start second node: %v", err)
	}

	defer func() { _ = second.Node.Stop(context.Background()) }()

	item, err := second.Workload.Get("work.echo")
	require.NoErrorf(t, err, "get workload after restart: %v", err)
	require.Falsef(t, item.Observed != "running", "observed = %q, want running", item.Observed)
	require.Falsef(t, strings.Contains(item.Reason,

		"restart reconciliation after node restart",
	), "unexpected restart recovery marker after graceful stop: %q", item.Reason)

}

func TestWorkloadNodeFailureWithoutHostedServiceImpactKeepsReady(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "work-ready-without-service-impact",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.invalid",
			Kind:    "unsupported",
			Owner:   "tenant",
			Desired: "running",
		}},
	})

	snap := n.Snapshot()
	require.Falsef(t, snap.Node.State != "ready" ||
		snap.Diag.Health.
			State !=
			"ready", "snapshot = %#v, want ready node and diagnostics", snap)

	items, err := testkit.Workloads(n).List()
	require.NoErrorf(t, err, "list workloads: %v", err)
	require.Falsef(t, len(items) != 1 || items[0].Observed != "failed", "workloads = %#v, want single failed workload", items)

}

func TestWorkloadNodeStopFailureDegradesWorkload(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "work-stop-fail",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	testkit.ReplaceWorkloadForIntegrationTest(harness, workloadcontroller.NewWithExecutorInDir(dir, nodeStopFailExecutor{}))
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	defer func() { _ = n.Stop(context.Background()) }()
	{

		err := harness.Workload.Register(context.Background(), workloadapi.SpecSnapshot{
			ID:      "work.stop.fail",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}
	{

		err := harness.Workload.Stop(context.Background(), "work.stop.fail")
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(), "workload stop failed"), "error = %v, want workload stop failed", err)
	}

	item, err := harness.Workload.Get("work.stop.fail")
	require.NoErrorf(t, err, "get workload: %v", err)
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedDegraded, "observed = %q, want degraded", item.Observed)

}

func TestWorkloadNodeInspectFailureWithdrawsPublication(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	exec := &nodeInspectFailureExecutor{}
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "work-inspect-fail",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.inspect",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.inspect",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://inspect:9000"},
			}},
		}},
	})
	n := harness.Node
	testkit.ReplaceWorkloadForIntegrationTest(harness, workloadcontroller.NewWithExecutorInDir(dir, exec))
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	defer func() { _ = n.Stop(context.Background()) }()

	exec.inspectErr = fmt.Errorf("temporary inspect failure")
	item, err := harness.Workload.Get("work.inspect")
	require.NoErrorf(t, err, "get workload: %v", err)
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedDegraded ||
		!strings.
			Contains(
				item.Reason,
				"inspect failed",
			), "item = %#v, want degraded inspect-failed truth", item)

	service, err := n.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service: %v", err)
	require.Falsef(t, service.Outcome != "not_found", "service outcome = %q, want not_found", service.Outcome)
	require.Falsef(t, testkit.Diagnostics(n).DiagnosticsSnapshot().
		Health.State != diagnostics.
		HealthDegraded, "health = %#v, want degraded", testkit.Diagnostics(n).DiagnosticsSnapshot().
		Health)

}

func TestWorkloadNodeDuplicateRegistrationPrefersConflictOverPolicy(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "work-duplicate-policy",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Policy: runtimeinfra.PolicyConfig{
			DeniedCapabilities: []string{"net-bind"},
		},
	})
	{

		err := testkit.Workloads(n).Register(context.Background(), workloadapi.SpecSnapshot{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Desired: "present",
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}
	{

		err := testkit.Workloads(n).Register(context.Background(), workloadapi.SpecSnapshot{
			ID:           "work.echo",
			Kind:         "service",
			Owner:        "node",
			Desired:      "present",
			Capabilities: []string{"net-bind"},
		})
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(), "already exists"), "error = %v, want already exists", err)
	}

	snap := n.Snapshot()
	require.Falsef(t, snap.Policy.State != "ready" ||
		snap.Policy.Reason !=
			"", "policy snapshot = %#v, want unchanged ready policy", snap.Policy)

}

func TestWorkloadNodeRollbackOnPublicationFailure(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	exec := &publicationRollbackExecutor{}
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "rollback-publication",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})
	n := harness.Node
	testkit.ReplaceWorkloadForIntegrationTest(harness, workloadcontroller.NewWithExecutorInDir(dir, exec))
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	defer func() { _ = n.Stop(context.Background()) }()
	{

		err := harness.Workload.Register(context.Background(), workloadapi.SpecSnapshot{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  "test",
			Desired: workloadregistry.DesiredStopped,
			Services: []workloadapi.PublishedServiceSnapshot{{
				ID:        "svc.work.echo",
				Type:      "echo",
				Owner:     "work.echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://echo:9000"},
			}},
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}
	{

		err := testkit.StopTransportForIntegrationTest(harness, context.Background())
		require.NoErrorf(t, err, "stop transport: %v", err)
	}
	{

		err := harness.Workload.Start(context.Background(), "work.echo")
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(), "workload start failed"), "error = %v, want workload start failed", err)
	}

	item, err := harness.Workload.Get("work.echo")
	require.NoErrorf(t, err, "get workload: %v", err)
	require.Falsef(t, item.Spec.Desired != workloadregistry.DesiredStopped ||
		item.
			Observed !=
			workloadcontroller.
				ObservedStopped, "item = %#v, want stopped rollback truth", item)
	require.Falsef(t, exec.stopCalls != 1, "stop calls = %d, want 1", exec.stopCalls)

	result, err := n.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service: %v", err)
	require.Falsef(t, len(result.Matches) !=
		0, "matches = %d, want 0 after rollback", len(result.Matches))

	found := false
	for _, item := range testkit.Diagnostics(n).DiagnosticsSnapshot().Health.Subsystems {
		if item.Domain == runtimepublication.Subsystem && item.State == diagnostics.HealthDegraded {
			found = true
			break
		}
	}
	require.True(t, found, "expected publication rollback diagnostics")

}

func TestWorkloadNodeReadPathsRefreshExitedTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "workload",
		ScenarioID:  "WKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "workload"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	exec := &nodeObservedExitExecutor{}
	exec.running.Store(true)
	endpoint, probe := startGenerationReadyServer(t, exec.generation.Load)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	harness := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "work-observed-exit", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: dir}, Privacy: privacy.Sender,
	})
	n := harness.Node
	testkit.ReplaceWorkloadForIntegrationTest(harness, workloadcontroller.NewWithExecutorInDir(dir, exec))
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	defer func() { _ = n.Stop(context.Background()) }()
	{

		err := harness.Workload.Register(context.Background(), workloadapi.SpecSnapshot{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  testkit.HelperProcessConfig(t, "sleep"),
			Desired: "running",
			Services: []workloadapi.PublishedServiceSnapshot{{
				ID:             "svc.work.echo",
				Type:           "echo",
				Owner:          "work.echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}
	testkit.WaitForServiceMatchCount(t, 10*time.Second, n, "echo", 1)

	before, err := n.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service before exit: %v", err)
	require.Falsef(t, len(before.Matches) !=
		1, "matches = %d, want 1 before exit", len(before.Matches))

	exec.running.Store(false)
	item, err := harness.Workload.Get("work.echo")
	require.NoErrorf(t, err, "get workload after exit: %v", err)
	require.Falsef(t, item.Observed != workloadcontroller.
		ObservedDegraded ||
		item.Instance.
			Running, "item = %#v, want degraded stopped instance", item)

	after, err := n.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service after exit: %v", err)
	require.Falsef(t, after.Outcome != "not_found" ||
		len(after.Matches) != 0, "result = %#v, want not_found after observed exit", after)
	require.Falsef(t, testkit.Diagnostics(n).DiagnosticsSnapshot().
		Health.State != diagnostics.
		HealthDegraded, "health = %#v, want degraded", testkit.Diagnostics(n).DiagnosticsSnapshot().
		Health)

}

type publicationRollbackExecutor struct {
	startCalls int
	stopCalls  int
}

type nodeStopFailExecutor struct{}

type nodeInspectFailureExecutor struct {
	startCalls int
	inspectErr error
}

type nodeObservedExitExecutor struct {
	running    atomic.Bool
	generation atomic.Int64
}

func (e *publicationRollbackExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{WorkloadID: "work.echo", Generation: time.Now().UTC().UnixNano(), PreparedAt: time.Now().UTC()}, nil
}

func (e *publicationRollbackExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	e.startCalls++
	return workloadcontroller.Instance{WorkloadID: "work.echo", Generation: time.Now().UTC().UnixNano(), Running: true, StartedAt: time.Now().UTC()}, nil
}

func (e *publicationRollbackExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	e.stopCalls++
	return nil
}

func (e *publicationRollbackExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.echo", Running: true, StartedAt: time.Now().UTC()}, nil
}

func (nodeStopFailExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{WorkloadID: "work.stop.fail", Generation: time.Now().UTC().UnixNano(), PreparedAt: time.Now().UTC()}, nil
}

func (nodeStopFailExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.stop.fail", Generation: time.Now().UTC().UnixNano(), Running: true, StartedAt: time.Now().UTC()}, nil
}

func (nodeStopFailExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	return fmt.Errorf("executor stop boom")
}

func (nodeStopFailExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.stop.fail", Running: true, StartedAt: time.Now().UTC()}, nil
}

func (e *nodeInspectFailureExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{WorkloadID: "work.inspect", Generation: time.Now().UTC().UnixNano(), PreparedAt: time.Now().UTC()}, nil
}

func (e *nodeInspectFailureExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	e.startCalls++
	return workloadcontroller.Instance{WorkloadID: "work.inspect", Generation: time.Now().UTC().UnixNano(), Running: true, StartedAt: time.Now().UTC()}, nil
}

func (e *nodeInspectFailureExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	return nil
}

func (e *nodeInspectFailureExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	if e.inspectErr != nil {
		return workloadcontroller.Instance{}, e.inspectErr
	}
	return workloadcontroller.Instance{WorkloadID: "work.inspect", Running: true, StartedAt: time.Now().UTC()}, nil
}

func (e *nodeObservedExitExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{WorkloadID: "work.echo", Generation: time.Now().UTC().UnixNano(), PreparedAt: time.Now().UTC()}, nil
}

func (e *nodeObservedExitExecutor) Start(_ context.Context, prepared workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	e.generation.Store(prepared.Generation)
	return workloadcontroller.Instance{WorkloadID: "work.echo", Generation: prepared.Generation, Running: true, PID: 4242, StartedAt: time.Now().UTC()}, nil
}

func (e *nodeObservedExitExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	e.running.Store(false)
	return nil
}

func (e *nodeObservedExitExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	if !e.running.Load() {
		return workloadcontroller.Instance{}, fmt.Errorf("workload work.echo not found")
	}
	return workloadcontroller.Instance{WorkloadID: "work.echo", Generation: e.generation.Load(), Running: true, PID: 4242, StartedAt: time.Now().UTC()}, nil
}

package execution

import (
	workloadregistry "ardents/internal/workload/registry"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadReconcilesAncillaryRuntimeForCurrentInstance(t *testing.T) {
	path := t.TempDir() + "/ardents.db"
	firstExecutor := &ancillaryRecoveryExecutor{}
	first := New(path, firstExecutor)
	require.NoError(t, first.Load())
	require.NoError(t, first.Register(workloadregistry.Spec{ID: "work.ingress", Kind: "service", Owner: "node", Desired: workloadregistry.DesiredRunning}))
	require.NoError(t, first.Reconcile(context.Background()))
	started, ok := first.Get("work.ingress")
	require.True(t, ok)

	recoveredExecutor := &ancillaryRecoveryExecutor{instance: started.Instance}
	recovered := New(path, recoveredExecutor)
	require.NoError(t, recovered.Load())
	require.Len(t, recoveredExecutor.reconciled, 1)
	require.Equal(t, started.Instance.RuntimeID, recoveredExecutor.reconciled[0].RuntimeID)
}

func TestRefreshObservedSupervisesAncillaryRuntimeWithBoundedBackoff(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	executor := &ancillaryRecoveryExecutor{
		instance: Instance{WorkloadID: "work.ingress", Generation: 1, RuntimeID: "runtime-current", Running: true},
	}
	service := New("", executor)
	service.now = func() time.Time { return now }
	require.NoError(t, service.Load())
	executor.failures = 1

	_, err := service.RefreshObserved(context.Background())
	require.ErrorContains(t, err, "ancillary runtime degraded")
	require.Equal(t, 2, executor.reconcileCalls)

	now = now.Add(500 * time.Millisecond)
	_, err = service.RefreshObserved(context.Background())
	require.ErrorContains(t, err, "ancillary runtime degraded")
	require.Equal(t, 2, executor.reconcileCalls, "retry ran before bounded backoff elapsed")

	now = now.Add(500 * time.Millisecond)
	_, err = service.RefreshObserved(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, executor.reconcileCalls)
}

func TestRefreshObservedRecoversKilledAncillaryForRunningWorkload(t *testing.T) {
	executor := &ancillaryRecoveryExecutor{
		instance:     Instance{WorkloadID: "work.ingress", Generation: 1, RuntimeID: "runtime-current", Running: true},
		proxyRunning: true,
	}
	service := New("", executor)
	require.NoError(t, service.Load())
	require.NoError(t, service.Register(workloadregistry.Spec{
		ID: "work.ingress", Kind: "service", Owner: "node", Desired: workloadregistry.DesiredRunning,
	}))
	require.NoError(t, service.Reconcile(context.Background()))

	executor.proxyRunning = false
	_, err := service.RefreshObserved(context.Background())
	require.NoError(t, err)
	require.True(t, executor.proxyRunning)
	require.Equal(t, 1, executor.proxyRecoveries)
	require.Equal(t, "runtime-current", executor.reconciled[0].RuntimeID)
}

type ancillaryRecoveryExecutor struct {
	instance        Instance
	reconciled      []Instance
	reconcileCalls  int
	failures        int
	proxyRunning    bool
	proxyRecoveries int
}

func (e *ancillaryRecoveryExecutor) Prepare(_ context.Context, req Request) (PreparedWorkload, error) {
	return PreparedWorkload{WorkloadID: req.WorkloadID, Generation: 1, PreparedAt: time.Now().UTC()}, nil
}

func (e *ancillaryRecoveryExecutor) Start(_ context.Context, prepared PreparedWorkload) (Instance, error) {
	e.instance = Instance{WorkloadID: prepared.WorkloadID, Generation: prepared.Generation,
		RuntimeID: "runtime-current", Running: true, StartedAt: time.Now().UTC()}
	return e.instance, nil
}

func (e *ancillaryRecoveryExecutor) Stop(context.Context, Instance) error { return nil }

func (e *ancillaryRecoveryExecutor) Inspect(context.Context, string) (Instance, error) {
	return e.instance, nil
}

func (e *ancillaryRecoveryExecutor) Managed(context.Context) ([]Instance, error) {
	if e.instance.WorkloadID == "" {
		return nil, nil
	}
	return []Instance{e.instance}, nil
}

func (e *ancillaryRecoveryExecutor) Remove(context.Context, Instance) error { return nil }

func (e *ancillaryRecoveryExecutor) ReconcileAncillary(_ context.Context, current []Instance) error {
	e.reconcileCalls++
	e.reconciled = append([]Instance(nil), current...)
	if e.failures > 0 {
		e.failures--
		return errors.New("proxy restart failed")
	}
	if len(current) > 0 && current[0].Running && !e.proxyRunning {
		e.proxyRunning = true
		e.proxyRecoveries++
	}
	return nil
}

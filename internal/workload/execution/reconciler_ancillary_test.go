package execution

import (
	workloadregistry "ardents/internal/workload/registry"
	"context"
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

type ancillaryRecoveryExecutor struct {
	instance   Instance
	reconciled []Instance
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
	e.reconciled = append([]Instance(nil), current...)
	return nil
}

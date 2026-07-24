package workload

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"ardents/internal/workload/execution"
	"ardents/internal/workload/registry"

	"github.com/stretchr/testify/require"
)

type boundedObservationExecutor struct {
	hang           atomic.Bool
	stopHang       atomic.Bool
	startHang      atomic.Bool
	managedHang    atomic.Bool
	entered        chan struct{}
	stopEntered    chan struct{}
	startEntered   chan struct{}
	managedEntered chan struct{}
}

func (e *boundedObservationExecutor) Prepare(_ context.Context, request execution.Request) (execution.PreparedWorkload, error) {
	return execution.PreparedWorkload{WorkloadID: request.WorkloadID, Generation: 1}, nil
}
func (e *boundedObservationExecutor) Start(ctx context.Context, prepared execution.PreparedWorkload) (execution.Instance, error) {
	if e.startHang.Load() {
		select {
		case e.startEntered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return execution.Instance{}, ctx.Err()
	}
	return execution.Instance{WorkloadID: prepared.WorkloadID, Generation: prepared.Generation, RuntimeID: "runtime-1", Running: true}, nil
}
func (e *boundedObservationExecutor) Stop(ctx context.Context, _ execution.Instance) error {
	if !e.stopHang.Load() {
		return nil
	}
	select {
	case e.stopEntered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}
func (e *boundedObservationExecutor) Inspect(ctx context.Context, workloadID string) (execution.Instance, error) {
	if e.hang.Load() {
		select {
		case e.entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return execution.Instance{}, ctx.Err()
	}
	return execution.Instance{WorkloadID: workloadID, Generation: 1, RuntimeID: "runtime-1", Running: true}, nil
}
func (e *boundedObservationExecutor) Managed(ctx context.Context) ([]execution.Instance, error) {
	if e.managedHang.Load() {
		select {
		case e.managedEntered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, nil
}
func (*boundedObservationExecutor) Remove(context.Context, execution.Instance) error { return nil }

type boundedObservationPublication struct{}

func (boundedObservationPublication) ProjectStatus(status execution.Status) execution.Status {
	return status
}
func (boundedObservationPublication) SyncDesired(context.Context) error                  { return nil }
func (boundedObservationPublication) SyncLocalDesired() error                            { return nil }
func (boundedObservationPublication) Capture() any                                       { return nil }
func (boundedObservationPublication) Rollback(context.Context, string, error, any) error { return nil }
func (boundedObservationPublication) HandleError(err error) error                        { return err }

func TestHungObservationReturnsBoundedExplicitlyDegradedCache(t *testing.T) {
	executor := newBoundedObservationExecutor()
	service := execution.New("", executor)
	require.NoError(t, service.Register(registry.Spec{
		ID: "work.cached", Kind: "service", Owner: "node", Desired: registry.DesiredRunning,
	}))
	require.NoError(t, service.Reconcile(context.Background()))
	now := time.Date(2032, 4, 5, 6, 7, 8, 0, time.UTC)
	runtime := NewRuntime(RuntimeConfig{
		Execution: service, Publication: boundedObservationPublication{},
		ObservationTimeout: 25 * time.Millisecond, ObservationMaxAge: time.Minute,
		Clock: func() time.Time { return now },
	})
	fresh, err := runtime.List()
	require.NoError(t, err)
	require.Len(t, fresh, 1)
	executor.hang.Store(true)

	result := make(chan struct {
		items []StatusSnapshot
		err   error
	}, 1)
	go func() {
		items, err := runtime.List()
		result <- struct {
			items []StatusSnapshot
			err   error
		}{items: items, err: err}
	}()
	<-executor.entered
	stateRead := make(chan bool, 1)
	go func() {
		_, ok := service.Get("work.cached")
		stateRead <- ok
	}()
	select {
	case ok := <-stateRead:
		require.True(t, ok)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("execution state mutex was held during external observation")
	}
	select {
	case observed := <-result:
		require.NoError(t, observed.err)
		require.Len(t, observed.items, 1)
		require.Equal(t, execution.ObservedDegraded, observed.items[0].Observed)
		require.Contains(t, observed.items[0].Reason, "cached Docker observation")
		require.True(t, observed.items[0].ObservationDegraded)
		require.Equal(t, now, observed.items[0].ObservedAt)
	case <-time.After(time.Second):
		t.Fatal("hung observation blocked cached workload query")
	}

	now = now.Add(2 * time.Minute)
	_, err = runtime.List()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded) || err.Error() != "")
}

func TestHungShutdownDoesNotHoldExecutionStateMutex(t *testing.T) {
	executor := newBoundedObservationExecutor()
	service := execution.New("", executor)
	require.NoError(t, service.Register(registry.Spec{
		ID: "work.shutdown", Kind: "service", Owner: "node", Desired: registry.DesiredRunning,
	}))
	require.NoError(t, service.Reconcile(context.Background()))
	runtime := NewRuntime(RuntimeConfig{Execution: service, Publication: boundedObservationPublication{}})
	executor.stopHang.Store(true)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	shutdown := make(chan error, 1)
	go func() { shutdown <- runtime.Shutdown(shutdownCtx) }()
	<-executor.stopEntered

	stateRead := make(chan bool, 1)
	go func() {
		_, ok := service.Get("work.shutdown")
		stateRead <- ok
	}()
	select {
	case ok := <-stateRead:
		require.True(t, ok)
	case <-time.After(25 * time.Millisecond):
		t.Fatal("execution state mutex was held during external shutdown")
	}
	require.Error(t, <-shutdown)
}

func TestHungReconcileDoesNotHoldExecutionStateMutex(t *testing.T) {
	executor := newBoundedObservationExecutor()
	executor.startHang.Store(true)
	service := execution.New("", executor)
	require.NoError(t, service.Register(registry.Spec{
		ID: "work.reconcile", Kind: "service", Owner: "node", Desired: registry.DesiredRunning,
	}))
	reconcileCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	reconciled := make(chan error, 1)
	go func() { reconciled <- service.Reconcile(reconcileCtx) }()
	<-executor.startEntered

	stateRead := make(chan bool, 1)
	go func() {
		_, ok := service.Get("work.reconcile")
		stateRead <- ok
	}()
	select {
	case ok := <-stateRead:
		require.True(t, ok)
	case <-time.After(25 * time.Millisecond):
		t.Fatal("execution state mutex was held during external reconcile")
	}
	require.NoError(t, <-reconciled)
}

func TestHungReconcileDoesNotPreventBoundedShutdown(t *testing.T) {
	executor := newBoundedObservationExecutor()
	service := execution.New("", executor)
	require.NoError(t, service.Register(registry.Spec{
		ID: "work.gated", Kind: "service", Owner: "node", Desired: registry.DesiredStopped,
	}))
	runtime := NewRuntime(RuntimeConfig{Execution: service, Publication: boundedObservationPublication{}})
	executor.startHang.Store(true)
	startCtx, cancelStart := context.WithCancel(context.Background())
	started := make(chan error, 1)
	go func() { started <- runtime.Start(startCtx, "work.gated") }()
	<-executor.startEntered

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancelShutdown()
	shutdownStarted := time.Now()
	err := runtime.Shutdown(shutdownCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(shutdownStarted), 250*time.Millisecond)

	cancelStart()
	require.Error(t, <-started)
}

func TestHungStartupInventoryDoesNotHoldExecutionStateMutex(t *testing.T) {
	executor := newBoundedObservationExecutor()
	executor.managedHang.Store(true)
	service := execution.New("", executor)
	loadCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	loaded := make(chan error, 1)
	go func() { loaded <- service.LoadContext(loadCtx) }()
	<-executor.managedEntered

	stateRead := make(chan struct{}, 1)
	go func() {
		service.List()
		stateRead <- struct{}{}
	}()
	select {
	case <-stateRead:
	case <-time.After(25 * time.Millisecond):
		t.Fatal("execution state mutex was held during startup inventory I/O")
	}
	require.ErrorIs(t, <-loaded, context.DeadlineExceeded)
}

func newBoundedObservationExecutor() *boundedObservationExecutor {
	return &boundedObservationExecutor{
		entered: make(chan struct{}, 1), stopEntered: make(chan struct{}, 1),
		startEntered: make(chan struct{}, 1), managedEntered: make(chan struct{}, 1),
	}
}

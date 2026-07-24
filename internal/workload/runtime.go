package workload

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"ardents/internal/workload/execution"
	"ardents/internal/workload/registry"
)

type AdmissionPolicy interface {
	AdmitWorkload(registry.Spec, []execution.Status) error
}

type PublicationPort interface {
	ProjectStatus(execution.Status) execution.Status
	SyncDesired(context.Context) error
	SyncLocalDesired() error
	Capture() any
	Rollback(context.Context, string, error, any) error
	HandleError(error) error
}

type RuntimeHooks struct {
	RefreshFailed     func(error)
	RefreshSucceeded  func()
	StateChanged      func([]execution.Status)
	EvaluateHealth    func([]execution.Status)
	ShutdownFailed    func(error)
	ShutdownSucceeded func()
	PolicyDenied      func(resource, action string, err error)
}

type RuntimeConfig struct {
	Execution          *execution.Service
	Policy             AdmissionPolicy
	Publication        PublicationPort
	Guard              func(action string) error
	Hooks              RuntimeHooks
	ObservationTimeout time.Duration
	ObservationMaxAge  time.Duration
	Clock              func() time.Time
}

type Runtime struct {
	mu            sync.Mutex
	operationGate chan struct{}
	cfg           RuntimeConfig
	cache         []StatusSnapshot
	cacheAt       time.Time
}

func NewRuntime(cfg RuntimeConfig) *Runtime {
	if cfg.ObservationTimeout <= 0 {
		cfg.ObservationTimeout = 2 * time.Second
	}
	if cfg.ObservationMaxAge <= 0 {
		cfg.ObservationMaxAge = 30 * time.Second
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Runtime{cfg: cfg, operationGate: make(chan struct{}, 1)}
}

func (r *Runtime) Load() error {
	return r.LoadContext(context.Background())
}

func (r *Runtime) LoadContext(ctx context.Context) error {
	if err := r.cfg.Execution.LoadContext(ctx); err != nil {
		return err
	}
	now := r.cfg.Clock().UTC()
	r.storeObservationCache(r.projectCurrent(now), now)
	return nil
}

func (r *Runtime) SeedAndReconcile(ctx context.Context, specs []registry.Spec) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := r.cfg.Execution.Seed(specs); err != nil {
		return err
	}
	return r.reconcile(ctx)
}

func (r *Runtime) List() ([]StatusSnapshot, error) {
	now := r.cfg.Clock().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.ObservationTimeout)
	err := r.syncObserved(ctx)
	cancel()
	if err == nil {
		items := r.projectCurrent(now)
		r.storeObservationCache(items, now)
		return cloneStatusSnapshots(items), nil
	}
	r.mu.Lock()
	cached, cachedAt := cloneStatusSnapshots(r.cache), r.cacheAt
	maxAge := r.cfg.ObservationMaxAge
	r.mu.Unlock()
	age := now.Sub(cachedAt)
	if cachedAt.IsZero() || age < 0 || age > maxAge {
		return nil, fmt.Errorf("workload observation unavailable and cache is stale: %w", err)
	}
	reason := fmt.Sprintf("cached Docker observation from %s: %v", cachedAt.Format(time.RFC3339), err)
	for index := range cached {
		cached[index].Observed = execution.ObservedDegraded
		cached[index].Reason = reason
		cached[index].NeedsOperatorAction = true
		cached[index].ObservationDegraded = true
		cached[index].ObservedAt = cachedAt
	}
	return cached, nil
}

func (r *Runtime) Get(id string) (StatusSnapshot, error) {
	items, err := r.List()
	if err != nil {
		return StatusSnapshot{}, err
	}
	for _, item := range items {
		if item.Spec.ID == id {
			return item, nil
		}
	}
	return StatusSnapshot{}, errors.New("workload not found")
}

func (r *Runtime) Register(ctx context.Context, spec SpecSnapshot) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := r.guard("workload register"); err != nil {
		return err
	}
	if _, exists := r.cfg.Execution.Get(spec.ID); exists {
		return fmt.Errorf("workload %s already exists", spec.ID)
	}
	model, err := SpecFromSnapshot(spec)
	if err != nil {
		return err
	}
	if err := r.cfg.Policy.AdmitWorkload(model, r.cfg.Execution.List()); err != nil {
		r.denied(spec.ID, "workload.register", err)
		return err
	}
	return r.mutate(ctx, "workload register", func() error { return r.cfg.Execution.Register(model) })
}

func (r *Runtime) Start(ctx context.Context, id string) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := r.guard("workload start"); err != nil {
		return err
	}
	return r.mutateDesired(ctx, id, registry.DesiredRunning, "workload start")
}

func (r *Runtime) Stop(ctx context.Context, id string) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := r.guard("workload stop"); err != nil {
		return err
	}
	return r.mutateDesired(ctx, id, registry.DesiredStopped, "workload stop")
}

func (r *Runtime) Restart(ctx context.Context, id string) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	if err := r.guard("workload restart"); err != nil {
		return err
	}
	if err := r.mutateDesired(ctx, id, registry.DesiredStopped, "workload restart stop"); err != nil {
		return err
	}
	return r.mutateDesired(ctx, id, registry.DesiredRunning, "workload restart start")
}

func (r *Runtime) SyncObserved(ctx context.Context) error {
	observationCtx, cancel := r.observationContext(ctx)
	defer cancel()
	return r.syncObserved(observationCtx)
}

func (r *Runtime) observationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, r.cfg.ObservationTimeout)
}

func (r *Runtime) syncObserved(ctx context.Context) error {
	cfg := r.cfg
	changed, err := cfg.Execution.RefreshObserved(ctx)
	if err != nil {
		r.refreshFailed(err)
	}
	if changed {
		r.stateChanged()
		if syncErr := cfg.Publication.SyncLocalDesired(); syncErr != nil {
			r.refreshFailed(syncErr)
			return errors.Join(err, syncErr)
		}
	}
	if err != nil {
		return err
	}
	if cfg.Hooks.RefreshSucceeded != nil {
		cfg.Hooks.RefreshSucceeded()
	}
	r.evaluateHealth()
	return nil
}

func (r *Runtime) Reconcile(ctx context.Context) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	return r.reconcile(ctx)
}

func (r *Runtime) reconcile(ctx context.Context) error {
	if err := r.guard("workload reconcile"); err != nil {
		return err
	}
	if err := r.cfg.Execution.Reconcile(ctx); err != nil {
		return err
	}
	r.stateChanged()
	if err := r.cfg.Publication.SyncDesired(ctx); err != nil {
		if handled := r.cfg.Publication.HandleError(err); handled != nil {
			return handled
		}
	}
	r.evaluateHealth()
	return nil
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	release, err := r.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()

	cfg := r.cfg
	if err := cfg.Execution.StopAll(ctx); err != nil {
		if cfg.Hooks.ShutdownFailed != nil {
			cfg.Hooks.ShutdownFailed(err)
		}
		return err
	}
	r.stateChanged()
	if err := cfg.Publication.SyncDesired(ctx); err != nil {
		if handled := cfg.Publication.HandleError(err); handled != nil {
			return fmt.Errorf("withdraw workload services: %w", handled)
		}
	}
	if cfg.Hooks.ShutdownSucceeded != nil {
		cfg.Hooks.ShutdownSucceeded()
	}
	return nil
}

func (r *Runtime) acquireOperation(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case r.operationGate <- struct{}{}:
		return func() { <-r.operationGate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *Runtime) projectCurrent(observedAt time.Time) []StatusSnapshot {
	items := r.cfg.Execution.List()
	out := make([]StatusSnapshot, 0, len(items))
	for _, item := range items {
		snapshot := ProjectStatus(r.cfg.Publication.ProjectStatus(item))
		snapshot.ObservedAt = observedAt
		out = append(out, snapshot)
	}
	return out
}

func (r *Runtime) storeObservationCache(items []StatusSnapshot, observedAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = cloneStatusSnapshots(items)
	r.cacheAt = observedAt
}

func cloneStatusSnapshots(items []StatusSnapshot) []StatusSnapshot {
	out := make([]StatusSnapshot, len(items))
	for index, item := range items {
		out[index] = item
		out[index].Spec.Services = clonePublishedServices(item.Spec.Services)
		out[index].Spec.Requirements = append([]registry.WorkloadRequirement(nil), item.Spec.Requirements...)
		out[index].PublishedServices = clonePublishedServices(item.PublishedServices)
	}
	return out
}

func clonePublishedServices(items []PublishedServiceSnapshot) []PublishedServiceSnapshot {
	out := make([]PublishedServiceSnapshot, len(items))
	for index, item := range items {
		out[index] = item
		out[index].Endpoints = append([]string(nil), item.Endpoints...)
		out[index].ProbeEndpoints = append([]string(nil), item.ProbeEndpoints...)
	}
	return out
}

func (r *Runtime) mutateDesired(ctx context.Context, id, desired, action string) error {
	if err := r.mutate(ctx, action, func() error { return r.cfg.Execution.SetDesired(id, desired) }); err != nil {
		return err
	}
	return r.requireOutcome(id, desired, action)
}

func (r *Runtime) mutate(ctx context.Context, action string, change func() error) error {
	snapshot := r.cfg.Publication.Capture()
	if err := change(); err != nil {
		return err
	}
	if err := r.reconcile(ctx); err != nil {
		if rollbackErr := r.cfg.Publication.Rollback(ctx, action, err, snapshot); rollbackErr != nil {
			return fmt.Errorf("%s failed: %w; rollback failed: %v", action, err, rollbackErr)
		}
		return fmt.Errorf("%s failed: %w", action, err)
	}
	return nil
}

func (r *Runtime) requireOutcome(id, desired, action string) error {
	item, ok := r.cfg.Execution.Get(id)
	if !ok {
		return fmt.Errorf("workload %s not found after reconcile", id)
	}
	valid := desired == registry.DesiredRunning && item.Observed == execution.ObservedRunning
	valid = valid || (desired == registry.DesiredStopped || desired == registry.DesiredDisabled) && item.Observed == execution.ObservedStopped
	if valid {
		return nil
	}
	if item.Reason != "" {
		return fmt.Errorf("%s failed: observed %s: %s", action, item.Observed, item.Reason)
	}
	return fmt.Errorf("%s failed: observed %s", action, item.Observed)
}

func (r *Runtime) guard(action string) error {
	if r.cfg.Guard != nil {
		return r.cfg.Guard(action)
	}
	return nil
}

func (r *Runtime) denied(resource, action string, err error) {
	if r.cfg.Hooks.PolicyDenied != nil {
		r.cfg.Hooks.PolicyDenied(resource, action, err)
	}
}

func (r *Runtime) refreshFailed(err error) {
	if r.cfg.Hooks.RefreshFailed != nil {
		r.cfg.Hooks.RefreshFailed(err)
	}
}

func (r *Runtime) stateChanged() {
	if r.cfg.Hooks.StateChanged != nil {
		r.cfg.Hooks.StateChanged(r.cfg.Execution.List())
	}
}

func (r *Runtime) evaluateHealth() {
	if r.cfg.Hooks.EvaluateHealth != nil {
		r.cfg.Hooks.EvaluateHealth(r.cfg.Execution.List())
	}
}

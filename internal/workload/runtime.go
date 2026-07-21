package workload

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
	Execution   *execution.Service
	Policy      AdmissionPolicy
	Publication PublicationPort
	Guard       func(action string) error
	Hooks       RuntimeHooks
}

type Runtime struct {
	mu  sync.Mutex
	cfg RuntimeConfig
}

func NewRuntime(cfg RuntimeConfig) *Runtime { return &Runtime{cfg: cfg} }

func (r *Runtime) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg.Execution.Load()
}

func (r *Runtime) SeedAndReconcile(ctx context.Context, specs []registry.Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.cfg.Execution.Seed(specs); err != nil {
		return err
	}
	return r.reconcile(ctx)
}

func (r *Runtime) List() ([]StatusSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.syncObserved(context.Background()); err != nil {
		return nil, err
	}
	items := r.cfg.Execution.List()
	out := make([]StatusSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, ProjectStatus(r.cfg.Publication.ProjectStatus(item)))
	}
	return out, nil
}

func (r *Runtime) Get(id string) (StatusSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.syncObserved(context.Background()); err != nil {
		return StatusSnapshot{}, err
	}
	item, ok := r.cfg.Execution.Get(id)
	if !ok {
		return StatusSnapshot{}, errors.New("workload not found")
	}
	return ProjectStatus(r.cfg.Publication.ProjectStatus(item)), nil
}

func (r *Runtime) Register(ctx context.Context, spec SpecSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.guard("workload register"); err != nil {
		return err
	}
	if _, exists := r.cfg.Execution.Get(spec.ID); exists {
		return fmt.Errorf("workload %s already exists", spec.ID)
	}
	model := SpecFromSnapshot(spec)
	if err := r.cfg.Policy.AdmitWorkload(model, r.cfg.Execution.List()); err != nil {
		r.denied(spec.ID, "workload.register", err)
		return err
	}
	return r.mutate(ctx, "workload register", func() error { return r.cfg.Execution.Register(model) })
}

func (r *Runtime) Start(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.guard("workload start"); err != nil {
		return err
	}
	return r.mutateDesired(ctx, id, registry.DesiredRunning, "workload start")
}

func (r *Runtime) Stop(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.guard("workload stop"); err != nil {
		return err
	}
	return r.mutateDesired(ctx, id, registry.DesiredStopped, "workload stop")
}

func (r *Runtime) Restart(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.guard("workload restart"); err != nil {
		return err
	}
	if err := r.mutateDesired(ctx, id, registry.DesiredStopped, "workload restart stop"); err != nil {
		return err
	}
	return r.mutateDesired(ctx, id, registry.DesiredRunning, "workload restart start")
}

func (r *Runtime) SyncObserved(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.syncObserved(ctx)
}

func (r *Runtime) syncObserved(ctx context.Context) error {
	changed, err := r.cfg.Execution.RefreshObserved(ctx)
	if err != nil {
		r.refreshFailed(err)
		return err
	}
	if changed {
		r.stateChanged()
		if err := r.cfg.Publication.SyncLocalDesired(); err != nil {
			r.refreshFailed(err)
			return err
		}
	}
	if r.cfg.Hooks.RefreshSucceeded != nil {
		r.cfg.Hooks.RefreshSucceeded()
	}
	r.evaluateHealth()
	return nil
}

func (r *Runtime) Reconcile(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.cfg.Execution.StopAll(ctx); err != nil {
		if r.cfg.Hooks.ShutdownFailed != nil {
			r.cfg.Hooks.ShutdownFailed(err)
		}
		return err
	}
	r.stateChanged()
	if err := r.cfg.Publication.SyncDesired(ctx); err != nil {
		if handled := r.cfg.Publication.HandleError(err); handled != nil {
			return fmt.Errorf("withdraw workload services: %w", handled)
		}
	}
	if r.cfg.Hooks.ShutdownSucceeded != nil {
		r.cfg.Hooks.ShutdownSucceeded()
	}
	return nil
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

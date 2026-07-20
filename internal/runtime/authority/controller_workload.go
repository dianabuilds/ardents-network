package authority

import (
	"context"
	"errors"
	"fmt"

	controlprojection "ardents/internal/control/projection"
	"ardents/internal/diagnostics"
	workloadapi "ardents/internal/workload/api"
	"ardents/internal/workload/desiredstate"
	"ardents/internal/workload/observedstate"
)

func (c *Controller) ListWorkloadsLocked() ([]workloadapi.WorkloadStatusSnapshot, error) {
	if err := c.SyncObservedWorkloadsLocked(context.Background()); err != nil {
		return nil, err
	}
	items := c.workload.List()
	out := make([]workloadapi.WorkloadStatusSnapshot, 0, len(items))
	for _, item := range items {
		item = effectiveWorkloadStatus(item, c.policy)
		out = append(out, controlprojection.WorkloadSnapshot(item))
	}
	return out, nil
}

func (c *Controller) GetWorkloadLocked(id string) (workloadapi.WorkloadStatusSnapshot, error) {
	if err := c.SyncObservedWorkloadsLocked(context.Background()); err != nil {
		return workloadapi.WorkloadStatusSnapshot{}, err
	}
	item, ok := c.workload.Get(id)
	if !ok {
		return workloadapi.WorkloadStatusSnapshot{}, errors.New("workload not found")
	}
	item = effectiveWorkloadStatus(item, c.policy)
	return controlprojection.WorkloadSnapshot(item), nil
}

func (c *Controller) SyncObservedWorkloadsLocked(ctx context.Context) error {
	changed, err := c.workload.RefreshObserved(ctx)
	if err != nil {
		c.recordWorkloadRefreshFailureLocked(err)
		return err
	}
	if !changed {
		c.clearWorkloadRefreshFailureLocked()
		c.evaluateWorkloadHealthLocked()
		return nil
	}
	c.refreshWorkloadStateLocked()
	if err := c.publication.SyncLocalDesiredLocked(); err != nil {
		c.recordWorkloadRefreshFailureLocked(err)
		return err
	}
	c.clearWorkloadRefreshFailureLocked()
	c.evaluateWorkloadHealthLocked()
	return nil
}

func (c *Controller) RegisterWorkloadLocked(ctx context.Context, spec workloadapi.WorkloadSpecSnapshot) error {
	if err := c.requireWorkloadRuntimeMutableLocked("workload register"); err != nil {
		return err
	}
	if _, exists := c.workload.Get(spec.ID); exists {
		return fmt.Errorf("workload %s already exists", spec.ID)
	}
	workloadSpec := workloadSpecFromAPI(spec)
	if err := c.policy.AdmitWorkload(workloadSpec, c.workload.List()); err != nil {
		c.policyDeniedLocked(spec.ID, "workload.register", err)
		return err
	}
	return c.mutateWorkloadWithRollbackLocked(ctx, "workload register", func() error {
		if err := c.workload.Register(workloadSpec); err != nil {
			return err
		}
		c.publish("workload.registered", map[string]any{"id": spec.ID})
		return nil
	})
}

func (c *Controller) StartWorkloadLocked(ctx context.Context, id string) error {
	if err := c.requireWorkloadRuntimeMutableLocked("workload start"); err != nil {
		return err
	}
	return c.mutateWorkloadDesiredLocked(ctx, id, desiredstate.Running, "workload start")
}

func (c *Controller) StopWorkloadLocked(ctx context.Context, id string) error {
	if err := c.requireWorkloadRuntimeMutableLocked("workload stop"); err != nil {
		return err
	}
	return c.mutateWorkloadDesiredLocked(ctx, id, desiredstate.Stopped, "workload stop")
}

func (c *Controller) RestartWorkloadLocked(ctx context.Context, id string) error {
	if err := c.requireWorkloadRuntimeMutableLocked("workload restart"); err != nil {
		return err
	}
	if err := c.mutateWorkloadDesiredLocked(ctx, id, desiredstate.Stopped, "workload restart stop"); err != nil {
		return err
	}
	return c.mutateWorkloadDesiredLocked(ctx, id, desiredstate.Running, "workload restart start")
}

func (c *Controller) ReconcileWorkloadsLocked(ctx context.Context) error {
	if err := c.requireWorkloadRuntimeMutableLocked("workload reconcile"); err != nil {
		return err
	}
	if err := c.workload.Reconcile(ctx); err != nil {
		return err
	}
	c.refreshWorkloadStateLocked()
	if err := c.publication.SyncDesiredLocked(ctx); err != nil {
		if handledErr := c.handlePublicationErrorLocked(err); handledErr != nil {
			return handledErr
		}
	}
	c.evaluateWorkloadHealthLocked()
	return nil
}

func (c *Controller) ShutdownWorkloadsLocked(ctx context.Context) error {
	if err := c.workload.StopAll(ctx); err != nil {
		c.diag.SetSubsystem("workload", diagnostics.HealthFailed, &diagnostics.Reason{
			Code:                   "workload.shutdown.failed",
			Domain:                 "workload",
			Summary:                "workload shutdown failed",
			Detail:                 err.Error(),
			Impact:                 "node stop could not fully terminate local workloads",
			Recovery:               "operator",
			OperatorActionRequired: true,
		})
		return err
	}
	c.refreshWorkloadStateLocked()
	if err := c.publication.SyncDesiredLocked(ctx); err != nil {
		if handledErr := c.handlePublicationErrorLocked(err); handledErr != nil {
			return fmt.Errorf("withdraw workload services: %w", handledErr)
		}
	}
	c.diag.ClearSubsystem("workload")
	return nil
}

func (c *Controller) mutateWorkloadDesiredLocked(ctx context.Context, id, desired, action string) error {
	if err := c.mutateWorkloadWithRollbackLocked(ctx, action, func() error {
		return c.workload.SetDesired(id, desired)
	}); err != nil {
		return err
	}
	return c.requireWorkloadOutcomeLocked(id, desired, action)
}

func (c *Controller) requireWorkloadOutcomeLocked(id, desired, action string) error {
	item, ok := c.workload.Get(id)
	if !ok {
		return fmt.Errorf("workload %s not found after reconcile", id)
	}
	switch desired {
	case desiredstate.Running:
		if item.Observed == observedstate.Running {
			return nil
		}
	case desiredstate.Stopped, desiredstate.Disabled:
		if item.Observed == observedstate.Stopped {
			return nil
		}
	default:
		return nil
	}
	if item.Reason != "" {
		return fmt.Errorf("%s failed: observed %s: %s", action, item.Observed, item.Reason)
	}
	return fmt.Errorf("%s failed: observed %s", action, item.Observed)
}

func (c *Controller) mutateWorkloadWithRollbackLocked(ctx context.Context, action string, mutate func() error) error {
	snapshot := c.publication.CaptureWorkloadPublicationSnapshotLocked()
	if err := mutate(); err != nil {
		return err
	}
	if err := c.ReconcileWorkloadsLocked(ctx); err != nil {
		rollbackErr := c.publication.RollbackWorkloadMutationLocked(ctx, action, err, snapshot)
		if rollbackErr != nil {
			return fmt.Errorf("%s failed: %w; rollback failed: %v", action, err, rollbackErr)
		}
		return fmt.Errorf("%s failed: %w", action, err)
	}
	return nil
}

func (c *Controller) refreshWorkloadStateLocked() {
	for _, item := range c.workload.List() {
		c.publish("workload.updated", map[string]any{
			"id":       item.Spec.ID,
			"observed": item.Observed,
			"desired":  item.Spec.Desired,
		})
	}
}

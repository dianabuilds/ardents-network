package execution

import (
	db "ardents/internal/storage"
	workloadregistry "ardents/internal/workload/registry"
	"context"
	"fmt"
	"time"
)

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return db.SaveJSON(s.path, "workload", "snapshot", persistentState{Items: s.items})
}

const runtimeInventoryTimeout = 30 * time.Second

func (s *Service) Snapshot() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SnapshotStatuses(s.items)
}

func (s *Service) Restore(ctx context.Context, snapshot []Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	target := make(map[string]Status, len(snapshot))
	for _, item := range snapshot {
		target[item.Spec.ID] = NormalizeStatus(CloneStatus(item))
	}
	for _, item := range s.items {
		targetItem, ok := target[item.Spec.ID]
		if !item.Instance.Running {
			continue
		}
		if ok && targetItem.Spec.Desired == workloadregistry.DesiredRunning {
			continue
		}
		if err := s.executor.Stop(ctx, item.Instance); err != nil {
			return err
		}
	}
	s.items = target
	now := time.Now().UTC()
	for id, item := range s.items {
		next, keep, err := s.reconcileLocked(ctx, item, now)
		if err != nil {
			return err
		}
		if keep {
			s.items[id] = next
			continue
		}
		delete(s.items, id)
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) recoverLoadedProcessesLocked() error {
	now := time.Now().UTC()
	for id, item := range s.items {
		if !item.Instance.Running {
			continue
		}
		current, inspectErr := s.executor.Inspect(context.Background(), id)
		if inspectErr == nil && current.Running && current.Generation == item.Instance.Generation {
			item.Instance = current
			s.items[id] = recoveredRunning(item, now)
			continue
		}
		if inspectNotFound(inspectErr) && ProcessMatchesConfig(item.Instance.PID, item.Spec.Config) {
			s.items[id] = recoveredRunning(item, now)
			continue
		}
		s.items[id] = recoveredStopped(item, current, inspectErr, now)
	}
	return nil
}

func (s *Service) reconcileRuntimeInventoryLocked() error {
	inventory, ok := s.executor.(Inventory)
	if !ok {
		return nil
	}
	remover, ok := s.executor.(Remover)
	if !ok {
		return fmt.Errorf("runtime inventory requires controlled removal support")
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtimeInventoryTimeout)
	defer cancel()
	managed, err := inventory.Managed(ctx)
	if err != nil {
		return fmt.Errorf("list managed workload instances: %w", err)
	}
	for _, instance := range managed {
		if s.isCurrentInstance(instance) {
			continue
		}
		if instance.Running {
			if err := s.executor.Stop(ctx, instance); err != nil {
				return fmt.Errorf("stop orphan workload %s: %w", instance.WorkloadID, err)
			}
		}
		if err := remover.Remove(ctx, instance); err != nil {
			return fmt.Errorf("remove orphan workload %s: %w", instance.WorkloadID, err)
		}
	}
	return s.reconcileAncillaryLocked(ctx)
}

func (s *Service) reconcileAncillaryLocked(ctx context.Context) error {
	reconciler, ok := s.executor.(AncillaryReconciler)
	if !ok {
		return nil
	}
	current := make([]Instance, 0, len(s.items))
	for _, item := range s.items {
		if item.Instance.WorkloadID != "" {
			current = append(current, item.Instance)
		}
	}
	if err := reconciler.ReconcileAncillary(ctx, current); err != nil {
		return fmt.Errorf("reconcile runtime ancillary instances: %w", err)
	}
	return nil
}

func (s *Service) isCurrentInstance(instance Instance) bool {
	item, ok := s.items[instance.WorkloadID]
	return ok && item.Instance.Generation == instance.Generation &&
		(item.Instance.RuntimeID == "" || item.Instance.RuntimeID == instance.RuntimeID)
}

func recoveredRunning(item Status, now time.Time) Status {
	item.Observed = ObservedRunning
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.LastTransitionAt = now
	item.PublishedServices = ServiceStatuses(item.Spec, true, "")
	return NormalizeStatus(item)
}

func recoveredStopped(item Status, current Instance, inspectErr error, now time.Time) Status {
	item.Instance.Running = false
	item.Instance.PID = 0
	if item.Instance.FinishedAt.IsZero() {
		item.Instance.FinishedAt = now
	}
	if item.Spec.Desired == workloadregistry.DesiredRunning {
		item.Observed = ObservedDegraded
		item.Reason = recoveryFailureReason(inspectErr, current, item.Instance.Generation)
		item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
	} else {
		item.Observed = ObservedStopped
		item.Reason = ""
		item.PublishedServices = ServiceStatuses(item.Spec, false, "workload not running")
	}
	item.LastTransitionAt = now
	return NormalizeStatus(item)
}

func recoveryFailureReason(err error, current Instance, expectedGeneration int64) string {
	if err != nil && !inspectNotFound(err) {
		return "runtime inspection failed after node restart: " + err.Error()
	}
	if current.WorkloadID != "" && current.Generation != expectedGeneration {
		return "runtime generation changed after node restart and requires reconciliation"
	}
	return "runtime instance was not found after node restart and requires restart reconciliation"
}

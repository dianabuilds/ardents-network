package controller

import (
	"context"
	"strings"
	"time"

	"ardents/internal/workload/execution"
	workloadregistry "ardents/internal/workload/registry"
)

func (s *Service) reconcileLocked(ctx context.Context, item Status, now time.Time) (Status, bool, error) {
	item = workloadregistry.NormalizeStatus(item)

	if err := workloadregistry.ValidateSpec(item.Spec); err != nil {
		return RejectAdmission(item, now, err), true, nil
	}
	if s.admission != nil {
		if err := s.admission(item.Spec, workloadregistry.SnapshotStatuses(s.items)); err != nil {
			return RejectAdmission(item, now, err), true, nil
		}
	}

	switch item.Spec.Desired {
	case DesiredPresent:
		return ReconcilePresent(item, now), true, nil
	case DesiredRunning:
		return s.reconcileRunning(ctx, item, now), true, nil
	case DesiredStopped, DesiredDisabled:
		return s.reconcileStopped(ctx, item, now), true, nil
	case DesiredRemoved:
		return s.reconcileRemoved(ctx, item, now)
	default:
		item.Observed = ObservedFailed
		item.Reason = "invalid desired state"
		item.NeedsOperatorAction = true
		item.LastTransitionAt = now
		item.PublishedServices = serviceStatuses(item.Spec, false, item.Reason)
		return item, true, nil
	}
}

func (s *Service) reconcileRunning(ctx context.Context, item Status, now time.Time) Status {
	if item.Observed == ObservedFailed && item.NeedsOperatorAction && item.Instance.WorkloadID != "" {
		return item
	}
	if next, ok := s.reconcileExistingProcess(ctx, item, now); ok {
		return next
	}
	if next, ok := s.removePreviousInstance(ctx, item, now); ok {
		return next
	}

	item.Observed = ObservedPreparing
	item.Reason = ""
	item.LastTransitionAt = now
	prepared, err := s.executor.Prepare(ctx, item.Spec)
	if err != nil {
		return s.failStart(item, now, "prepare failed: "+err.Error())
	}
	instance, err := s.executor.Start(ctx, prepared)
	if err != nil {
		return s.failStart(item, now, "start failed: "+err.Error())
	}
	item.Instance = instance
	item.Observed = ObservedRunning
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.LastTransitionAt = now
	item.PublishedServices = serviceStatuses(item.Spec, true, "")
	return MarkRunning(item, now)
}

func (s *Service) removePreviousInstance(ctx context.Context, item Status, now time.Time) (Status, bool) {
	if item.Instance.WorkloadID == "" || item.Instance.Running {
		return item, false
	}
	remover, ok := s.executor.(Remover)
	if !ok {
		item.Instance = Instance{}
		return item, false
	}
	if err := remover.Remove(ctx, item.Instance); err != nil {
		return s.failStart(item, now, "previous instance cleanup failed: "+err.Error()), true
	}
	item.Instance = Instance{}
	return item, false
}

func (s *Service) reconcileExistingProcess(ctx context.Context, item Status, now time.Time) (Status, bool) {
	if item.Instance.WorkloadID == "" || !item.Instance.Running {
		return item, false
	}
	current, err := s.executor.Inspect(ctx, item.Spec.ID)
	if next, ok := keepRunningProcess(item, current, err, now); ok {
		return next, true
	}
	if err == nil {
		item.Instance = current
	}
	return s.observedAfterUnexpectedExit(item, now), true
}

func keepRunningProcess(item Status, current Instance, err error, now time.Time) (Status, bool) {
	switch {
	case err == nil && current.Running:
		if next, ok := DegradedRunningAgainstDesired(item, current, now); ok {
			return next, true
		}
		item.Instance = current
		return MarkRunning(item, now), true
	case inspectNotFound(err) && execution.ProcessRunning(item.Instance.PID):
		if next, ok := DegradedRunningAgainstDesired(item, item.Instance, now); ok {
			return next, true
		}
		return MarkRunning(item, now), true
	case err != nil && !inspectNotFound(err):
		item.Observed = ObservedDegraded
		item.Reason = "inspect failed: " + err.Error()
		item.NeedsOperatorAction = true
		item.LastTransitionAt = now
		item.PublishedServices = serviceStatuses(item.Spec, false, item.Reason)
		return item, true
	default:
		return item, false
	}
}

func inspectNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func (s *Service) reconcileStopped(ctx context.Context, item Status, now time.Time) Status {
	if item.Instance.WorkloadID != "" && item.Instance.Running {
		item.Observed = ObservedStopping
		item.LastTransitionAt = now
		if err := s.executor.Stop(ctx, item.Instance); err != nil {
			item.Observed = ObservedDegraded
			item.Reason = "stop failed: " + err.Error()
			item.NeedsOperatorAction = true
			item.LastTransitionAt = now
			item.PublishedServices = serviceStatuses(item.Spec, false, item.Reason)
			return item
		}
		stopped, err := s.executor.Inspect(ctx, item.Spec.ID)
		if err == nil {
			item.Instance = stopped
		} else {
			item.Instance.Running = false
			item.Instance.FinishedAt = now
			item.Instance.Reason = "stopped"
		}
	}
	item.Observed = ObservedStopped
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.LastTransitionAt = now
	item.PublishedServices = serviceStatuses(item.Spec, false, "workload not running")
	return item
}

func (s *Service) reconcileRemoved(ctx context.Context, item Status, now time.Time) (Status, bool, error) {
	if item.Instance.WorkloadID != "" && item.Instance.Running {
		if err := s.executor.Stop(ctx, item.Instance); err != nil {
			item.Observed = ObservedDegraded
			item.Reason = "remove stop failed: " + err.Error()
			item.NeedsOperatorAction = true
			item.PublishedServices = serviceStatuses(item.Spec, false, item.Reason)
			return item, true, nil
		}
	}
	if remover, ok := s.executor.(Remover); ok && item.Instance.WorkloadID != "" {
		if err := remover.Remove(ctx, item.Instance); err != nil {
			item.Observed = ObservedDegraded
			item.Reason = "remove failed: " + err.Error()
			item.NeedsOperatorAction = true
			item.PublishedServices = serviceStatuses(item.Spec, false, item.Reason)
			return item, true, nil
		}
	}
	item.Observed = ObservedRemoved
	item.Reason = ""
	item.Instance = Instance{}
	item.LastTransitionAt = now
	item.PublishedServices = nil
	return item, false, nil
}

func (s *Service) failStart(item Status, now time.Time, reason string) Status {
	item.RestartCount++
	item.Reason = reason
	item.LastTransitionAt = now
	item.PublishedServices = serviceStatuses(item.Spec, false, reason)
	item.Instance = Instance{
		WorkloadID: item.Spec.ID,
		Generation: item.Instance.Generation + 1,
		Running:    false,
		FinishedAt: now,
		Reason:     reason,
	}
	if item.RestartCount > s.restartBudget || item.Spec.RestartPolicy == "never" {
		item.Observed = ObservedFailed
		item.NeedsOperatorAction = true
		return item
	}
	item.Observed = ObservedDegraded
	item.NeedsOperatorAction = false
	return item
}

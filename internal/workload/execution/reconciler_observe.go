package execution

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

func (s *Service) RefreshObserved(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	changed := false
	for id, item := range s.items {
		next := s.observeLocked(ctx, item, now)
		if reflect.DeepEqual(item, next) {
			continue
		}
		s.items[id] = next
		changed = true
	}
	if !changed {
		return false, nil
	}
	return true, s.saveLocked()
}

func (s *Service) observeLocked(ctx context.Context, item Status, now time.Time) Status {
	item = NormalizeStatus(item)
	if item.Instance.WorkloadID == "" {
		return item
	}
	if !item.Instance.Running {
		if item.Observed == ObservedRunning || item.Observed == ObservedPreparing {
			return s.observedAfterUnexpectedExit(item, now)
		}
		return item
	}

	current, err := s.executor.Inspect(ctx, item.Spec.ID)
	switch {
	case err == nil && current.Running:
		if next, ok := DegradedRunningAgainstDesired(item, current, now); ok {
			return next
		}
		item.Instance = current
		return MarkRunning(item, now)
	case inspectNotFound(err) && ProcessMatchesConfig(item.Instance.PID, item.Spec.Config):
		if next, ok := DegradedRunningAgainstDesired(item, item.Instance, now); ok {
			return next
		}
		return MarkRunning(item, now)
	case err != nil && !inspectNotFound(err):
		item.Observed = ObservedDegraded
		item.Reason = "inspect failed: " + err.Error()
		item.NeedsOperatorAction = true
		item.LastTransitionAt = now
		item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
		return item
	default:
		if err == nil {
			item.Instance = current
		}
		return s.observedAfterUnexpectedExit(item, now)
	}
}

const nodeShutdownStopReason = "workload stopped by node shutdown"

func (s *Service) StopAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var firstErr error
	for id, item := range s.items {
		next, err := s.stopForNodeShutdownLocked(ctx, item, now)
		s.items[id] = NormalizeStatus(next)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		if firstErr != nil {
			return fmt.Errorf("%v; save workloads: %w", firstErr, err)
		}
		return err
	}
	return firstErr
}

func (s *Service) stopForNodeShutdownLocked(ctx context.Context, item Status, now time.Time) (Status, error) {
	item = NormalizeStatus(item)
	if item.Instance.WorkloadID != "" && item.Instance.Running {
		item.Observed = ObservedStopping
		item.LastTransitionAt = now
		if err := s.executor.Stop(ctx, item.Instance); err != nil {
			item.Observed = ObservedDegraded
			item.Reason = "node shutdown stop failed: " + err.Error()
			item.NeedsOperatorAction = true
			item.LastTransitionAt = now
			item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
			return item, fmt.Errorf("stop workload %s: %w", item.Spec.ID, err)
		}
		stopped, err := s.executor.Inspect(ctx, item.Spec.ID)
		if err == nil {
			item.Instance = stopped
		} else {
			item.Instance = stoppedInstanceAfterNodeShutdown(item.Instance, now)
		}
	}

	item.Instance = stoppedInstanceAfterNodeShutdown(item.Instance, now)
	item.Observed = ObservedStopped
	item.Reason = ""
	item.NeedsOperatorAction = false
	item.LastTransitionAt = now
	item.PublishedServices = ServiceStatuses(item.Spec, false, nodeShutdownStopReason)
	return item, nil
}

func stoppedInstanceAfterNodeShutdown(instance Instance, now time.Time) Instance {
	if instance.WorkloadID == "" {
		return instance
	}
	instance.Running = false
	instance.PID = 0
	if instance.FinishedAt.IsZero() {
		instance.FinishedAt = now
	}
	if instance.Reason == "" {
		instance.Reason = nodeShutdownStopReason
	}
	return instance
}

package controller

import (
	"context"
	"reflect"
	"time"

	"ardents/internal/workload/execution"
	workloadregistry "ardents/internal/workload/registry"
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
	item = workloadregistry.NormalizeStatus(item)
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
	case inspectNotFound(err) && execution.ProcessMatchesConfig(item.Instance.PID, item.Spec.Config):
		if next, ok := DegradedRunningAgainstDesired(item, item.Instance, now); ok {
			return next
		}
		return MarkRunning(item, now)
	case err != nil && !inspectNotFound(err):
		item.Observed = ObservedDegraded
		item.Reason = "inspect failed: " + err.Error()
		item.NeedsOperatorAction = true
		item.LastTransitionAt = now
		item.PublishedServices = serviceStatuses(item.Spec, false, item.Reason)
		return item
	default:
		if err == nil {
			item.Instance = current
		}
		return s.observedAfterUnexpectedExit(item, now)
	}
}

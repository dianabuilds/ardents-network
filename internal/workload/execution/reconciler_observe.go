package execution

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"
)

func (s *Service) RefreshObserved(ctx context.Context) (bool, error) {
	s.mu.Lock()
	now := s.now().UTC()
	workCopies, comparisonSnapshots := s.captureStatusesLocked()
	ancillaryWaitErr := s.ancillaryBackoff.waitError(now)
	ancillaryCurrent := make([]Instance, 0, len(s.items))
	for _, item := range s.items {
		if item.Instance.WorkloadID != "" && item.Instance.Running {
			ancillaryCurrent = append(ancillaryCurrent, item.Instance)
		}
	}
	s.mu.Unlock()

	nextItems := make(map[string]Status, len(workCopies))
	var observationErr error
	for id, item := range workCopies {
		next, err := s.observeSnapshot(ctx, item, now)
		nextItems[id] = next
		observationErr = errors.Join(observationErr, err)
	}
	var ancillaryErr error
	reconciler, hasAncillary := s.executor.(AncillaryReconciler)
	if hasAncillary {
		if ancillaryWaitErr != nil {
			ancillaryErr = ancillaryWaitErr
		} else {
			ancillaryErr = reconciler.ReconcileAncillary(ctx, ancillaryCurrent)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for id := range workCopies {
		current, found := s.items[id]
		if !found || !reflect.DeepEqual(current, comparisonSnapshots[id]) {
			continue
		}
		next := nextItems[id]
		if reflect.DeepEqual(current, next) {
			continue
		}
		s.items[id] = next
		changed = true
	}
	if hasAncillary && ancillaryWaitErr == nil {
		if ancillaryErr != nil {
			ancillaryErr = s.ancillaryBackoff.fail(now, ancillaryErr)
		} else {
			s.ancillaryBackoff.reset()
		}
	}
	if changed {
		if err := s.saveLocked(); err != nil {
			return true, errors.Join(observationErr, ancillaryErr, err)
		}
	}
	return changed, errors.Join(observationErr, ancillaryErr)
}

type ancillaryBackoff struct {
	failures    int
	retryAt     time.Time
	lastFailure error
}

func (b *ancillaryBackoff) waitError(now time.Time) error {
	if !now.Before(b.retryAt) {
		return nil
	}
	return fmt.Errorf("ancillary runtime degraded; retry after %s: %w",
		b.retryAt.Format(time.RFC3339Nano), b.lastFailure)
}

func (b *ancillaryBackoff) fail(now time.Time, failure error) error {
	b.failures++
	delay := time.Second << min(b.failures-1, 5)
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	b.retryAt = now.Add(delay)
	b.lastFailure = failure
	return fmt.Errorf("ancillary runtime degraded; retry after %s: %w",
		b.retryAt.Format(time.RFC3339Nano), failure)
}

func (b *ancillaryBackoff) reset() {
	*b = ancillaryBackoff{}
}

func (s *Service) observeSnapshot(ctx context.Context, item Status, now time.Time) (Status, error) {
	item = NormalizeStatus(item)
	if item.Instance.WorkloadID == "" {
		return item, nil
	}
	if !item.Instance.Running {
		if item.Observed == ObservedRunning || item.Observed == ObservedPreparing {
			return s.observedAfterUnexpectedExit(item, now), nil
		}
		return item, nil
	}

	current, err := s.executor.Inspect(ctx, item.Spec.ID)
	switch {
	case err == nil && current.Running:
		if next, ok := DegradedRunningAgainstDesired(item, current, now); ok {
			return next, nil
		}
		item.Instance = current
		return MarkRunning(item, now), nil
	case inspectNotFound(err) && ProcessMatchesConfig(item.Instance.PID, item.Spec.Config):
		if next, ok := DegradedRunningAgainstDesired(item, item.Instance, now); ok {
			return next, nil
		}
		return MarkRunning(item, now), nil
	case err != nil && !inspectNotFound(err):
		item.Observed = ObservedDegraded
		item.Reason = "inspect failed: " + err.Error()
		item.NeedsOperatorAction = true
		item.LastTransitionAt = now
		item.PublishedServices = ServiceStatuses(item.Spec, false, item.Reason)
		return item, err
	default:
		if err == nil {
			item.Instance = current
		}
		return s.observedAfterUnexpectedExit(item, now), nil
	}
}

const nodeShutdownStopReason = "workload stopped by node shutdown"

func (s *Service) StopAll(ctx context.Context) error {
	release, err := s.acquireOperation(ctx)
	if err != nil {
		return err
	}
	defer release()

	s.mu.Lock()
	now := s.now().UTC()
	workCopies, comparisonSnapshots := s.captureStatusesLocked()
	s.mu.Unlock()

	var firstErr error
	nextItems := make(map[string]Status, len(workCopies))
	for id, item := range workCopies {
		next, err := s.stopForNodeShutdown(ctx, item, now)
		nextItems[id] = NormalizeStatus(next)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range workCopies {
		current, found := s.items[id]
		if !found || !reflect.DeepEqual(current, comparisonSnapshots[id]) {
			continue
		}
		s.items[id] = nextItems[id]
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

func (s *Service) captureStatusesLocked() (workCopies, comparisonSnapshots map[string]Status) {
	workCopies = make(map[string]Status, len(s.items))
	comparisonSnapshots = make(map[string]Status, len(s.items))
	for id, item := range s.items {
		comparisonSnapshots[id] = item
		workCopies[id] = CloneStatus(item)
	}
	return workCopies, comparisonSnapshots
}

func (s *Service) stopForNodeShutdown(ctx context.Context, item Status, now time.Time) (Status, error) {
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

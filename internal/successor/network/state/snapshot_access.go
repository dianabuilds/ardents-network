package state

import (
	"errors"
	"fmt"
	"time"
)

// ErrNoCurrentGeneration identifies an owned State root which has not yet
// accepted its first authenticated generation. Callers with a configured
// finite Source plan may use this narrow condition to bootstrap synchronously.
var ErrNoCurrentGeneration = errors.New("network state has no current generation")

// Current returns a copy of the current immutable Snapshot.
func (s *networkState) Current() (Snapshot, error) {
	if s.config.permitWork != nil {
		if err := s.config.permitWork(); err != nil {
			return Snapshot{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.automaticErr != nil {
		return Snapshot{}, fmt.Errorf("automatic Network State refresh failed: %w", s.automaticErr)
	}
	if s.terminalErr != nil {
		return Snapshot{}, fmt.Errorf("network owner failed: %w", s.terminalErr)
	}
	if s.current == nil {
		return Snapshot{}, ErrNoCurrentGeneration
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil && s.config.sourceInfo.Configured {
		snapshot := s.snapshotWithDistribution(s.config.clock().UTC())
		snapshot.Freshness = "clock-uncertain"
		return snapshot, nil
	}
	if err != nil {
		now = s.config.clock().UTC()
	}
	return s.snapshotWithDistribution(now), nil
}

func (s *networkState) snapshotWithDistribution(now time.Time) Snapshot {
	if s.current == nil {
		return Snapshot{}
	}
	snapshot := snapshotFromEpoch(s.current.Snapshot)
	snapshot.Conflicting = s.distribution.conflicting
	snapshot.SourceAttempts = uint16(len(s.distribution.history))
	snapshot.LatestCompleteness = "latest completeness unproven"
	snapshot.ObservedEpochs = s.distribution.observedEpochs
	snapshot.ObservedDigests = s.distribution.observedDigests
	for index, outcome := range s.distribution.outcomes {
		snapshot.SourceOutcomes[index] = sourceOutcomeName(outcome)
	}
	if s.distribution.trustedTimeFloor != 0 {
		snapshot.TrustedTime = time.Unix(s.distribution.trustedTimeFloor, 0).UTC()
	}
	if s.distribution.nextAutomatic != 0 {
		snapshot.NextAutomatic = time.Unix(s.distribution.nextAutomatic, 0).UTC()
	}
	if s.pendingDecision != nil {
		snapshot.PendingEpoch = s.pendingDecision.Header.Number
		snapshot.PendingDigest = s.pendingDecision.Header.Digest
		snapshot.PendingAt = s.pendingDecision.Header.ValidFrom
	}
	switch {
	case snapshot.Conflicting:
		snapshot.Freshness = "conflicting"
	case now.Before(s.current.Header.ValidFrom):
		snapshot.Freshness = "staged"
	case !now.Before(snapshot.ValidUntil):
		snapshot.Freshness = "expired"
	default:
		snapshot.Freshness = "fresh"
	}
	return snapshot
}

package state

import (
	"errors"
	"fmt"
)

// ErrNoCurrentGeneration identifies an owned State root which has not yet
// accepted its first authenticated generation. Callers with a configured
// finite Source plan may use this narrow condition to bootstrap synchronously.
var ErrNoCurrentGeneration = errors.New("network state has no current generation")

// Current returns a copy of the current immutable Snapshot.
func (s *networkState) Current() (Snapshot, error) {
	if s.resourceGuard != nil {
		if err := s.resourceGuard.Check(); err != nil {
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
	if s.resourceErr != nil {
		return Snapshot{}, fmt.Errorf("H3-S resource governor failed: %w", s.resourceErr)
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

package networkstate

import (
	"errors"
	"fmt"
)

// Current returns a copy of the current immutable Snapshot.
func (s *store) Current() (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.automaticErr != nil {
		return Snapshot{}, fmt.Errorf("automatic Network State refresh failed: %w", s.automaticErr)
	}
	if s.current == nil {
		return Snapshot{}, errors.New("network state has no current generation")
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil && s.config.sources[0].address != "" {
		snapshot := s.snapshotWithDistribution(s.config.clock().UTC())
		snapshot.Freshness = "clock-uncertain"
		return snapshot, nil
	}
	if err != nil {
		now = s.config.clock().UTC()
	}
	return s.snapshotWithDistribution(now), nil
}

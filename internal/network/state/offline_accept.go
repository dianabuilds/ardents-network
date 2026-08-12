package state

import (
	"context"
	"errors"
)

// Accept verifies a complete offline decision before committing a new generation.
func (s *networkState) Accept(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.refreshing {
		return Snapshot{}, errors.New("network state refresh owns the active transition")
	}
	decision, err := verifyDecision(s.config, s.current, epoch, inputs, encodedMaterials, true)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	state := s.distribution
	state.sequence++
	state.trustedTimeFloor = max(state.trustedTimeFloor, s.config.clock().UTC().Unix())
	if err := s.commitActiveDecision(decision, state); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotWithDistribution(s.config.clock().UTC()), nil
}

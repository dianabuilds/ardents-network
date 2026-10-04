package state

import (
	"context"
	"errors"
)

// Accept verifies a complete offline decision before committing a new generation.
func (s *networkState) Accept(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte) (Snapshot, error) {
	return s.acceptWithConflictCommit(ctx, epoch, inputs, encodedMaterials, s.storage.CommitControl)
}

func (s *networkState) acceptWithConflictCommit(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte, commit func(string, []byte) error) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if s.config.permitWork != nil {
		if err := s.config.permitWork(); err != nil {
			return Snapshot{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.refreshing {
		return Snapshot{}, errors.New("network state refresh owns the active transition")
	}
	if s.distribution.conflicting {
		return Snapshot{}, errPersistentStateConflict
	}
	// One acceptance must verify and publish against the same clock sample.
	verification := s.config
	verification.now = verification.clock().UTC()
	decision, err := authenticateDecision(verification, epoch, inputs, encodedMaterials, true)
	if err != nil {
		return Snapshot{}, err
	}
	// The retired-schema refusal precedes every commit and durable effect (F-50).
	if err := requireClosedIntakeSchema(s.config, decision.Header); err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if err := s.allowCandidateTransition(decision); err != nil {
		if isEpochHistoryConflict(err) {
			state := s.distribution
			state.sequence++
			state.trustedTimeFloor = max(state.trustedTimeFloor, verification.now.Unix())
			state.conflicting = true
			if commitErr := s.commitDistributionWithControl(state, commit); commitErr != nil {
				return Snapshot{}, commitErr
			}
		}
		return Snapshot{}, err
	}
	state := s.distribution
	state.sequence++
	state.trustedTimeFloor = max(state.trustedTimeFloor, verification.now.Unix())
	if err := s.commitActiveDecision(decision, state); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotWithDistribution(verification.now), nil
}

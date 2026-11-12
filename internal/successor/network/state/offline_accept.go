package state

import (
	"context"
	"errors"

	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

var errOfflineEpochNotCurrent = errors.New("offline Epoch is not yet current")

// Accept verifies a complete offline decision before publishing current or pending
// State. Only a Source wave may promote a retained pending Epoch.
func (s *networkState) Accept(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte) (Snapshot, error) {
	result, err := s.AcceptEpoch(ctx, epoch, inputs, encodedMaterials)
	return result.Snapshot, err
}

// OfflineAcceptance distinguishes an accepted pending Epoch from a current
// Epoch. Snapshot is the resulting State, including the existing current
// Epoch when the supplied candidate was retained as pending.
type OfflineAcceptance struct {
	Snapshot Snapshot
	Pending  bool
}

// AcceptEpoch returns the disposition of the supplied candidate as well as
// the resulting State, so command receipts need not inspect the Epoch bytes.
func (s *networkState) AcceptEpoch(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte) (OfflineAcceptance, error) {
	return s.acceptWithConflictCommit(ctx, epoch, inputs, encodedMaterials, s.storage.CommitControl)
}

func (s *networkState) acceptWithConflictCommit(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte, commit func(string, []byte) error) (OfflineAcceptance, error) {
	if err := ctx.Err(); err != nil {
		return OfflineAcceptance{}, err
	}
	if s.config.permitWork != nil {
		if err := s.config.permitWork(); err != nil {
			return OfflineAcceptance{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return OfflineAcceptance{}, errors.New("network state is closed")
	}
	if s.refreshing {
		return OfflineAcceptance{}, errors.New("network state refresh owns the active transition")
	}
	if s.distribution.conflicting {
		return OfflineAcceptance{}, errPersistentStateConflict
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil {
		return OfflineAcceptance{}, err
	}
	header, err := epoch2.Inspect(epoch)
	if err != nil {
		return OfflineAcceptance{}, err
	}
	// Authenticate future bytes at the signed window's opening, as Source
	// intake does. This reference is never used as evidence of current time.
	verification := s.config
	verification.now = now
	if now.Before(header.ValidFrom) {
		verification.now = header.ValidFrom
	}
	decision, err := authenticateDecision(verification, epoch, inputs, encodedMaterials, true)
	if err != nil {
		return OfflineAcceptance{}, err
	}
	// The retired-schema refusal precedes every commit and durable effect (F-50).
	if err := requireClosedIntakeSchema(s.config, decision.Header); err != nil {
		return OfflineAcceptance{}, err
	}
	if err := ctx.Err(); err != nil {
		return OfflineAcceptance{}, err
	}
	_, selection, err := s.selectEpochHistory([]epoch2.Decision{decision})
	if err != nil {
		if isEpochHistoryConflict(err) {
			state := s.distribution
			state.sequence++
			state.conflicting = true
			if commitErr := s.commitDistributionWithControl(state, commit); commitErr != nil {
				return OfflineAcceptance{}, commitErr
			}
		}
		return OfflineAcceptance{}, err
	}

	evidence := sampleClockEvidence(s.config)
	confirmed, err := confirmClockEvidence(evidence, s.distribution)
	if err != nil {
		return OfflineAcceptance{}, err
	}
	now = confirmed.Instant()
	if err := ctx.Err(); err != nil {
		return OfflineAcceptance{}, err
	}
	// The confidence tolerance and retained floor cannot move activation
	// across ValidFrom while either corroborating observation is still behind.
	opening := evidence.Monotonic
	if evidence.Independent.Before(opening) {
		opening = evidence.Independent
	}
	pending, err := offlineEpochPending(selection, opening, now, decision.Header.ValidUntil)
	if err != nil {
		return OfflineAcceptance{}, err
	}
	if s.pendingDecision != nil && s.pendingDecision.Header.Digest == decision.Header.Digest {
		return OfflineAcceptance{Snapshot: s.snapshotWithDistribution(now), Pending: true}, nil
	}
	if s.current != nil && s.current.Header.Digest == decision.Header.Digest {
		if pending {
			return OfflineAcceptance{}, errOfflineEpochNotCurrent
		}
		return OfflineAcceptance{Snapshot: s.snapshotWithDistribution(now)}, nil
	}

	// Offline acceptance changes Epoch history, never the trusted time floor.
	state := s.distribution
	state.sequence++
	if pending {
		if err := s.commitPendingDecision(decision, state); err != nil {
			return OfflineAcceptance{}, err
		}
		return OfflineAcceptance{Snapshot: s.snapshotWithDistribution(now), Pending: true}, nil
	}
	if err := s.commitActiveDecision(decision, state); err != nil {
		return OfflineAcceptance{}, err
	}
	return OfflineAcceptance{Snapshot: s.snapshotWithDistribution(now)}, nil
}

package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// recoverDistributionHistory checks the signed current chain first, then
// reconciles it with the journal on the same exclusive lease. This runs before
// State publishes an observation, starts serving, or admits refresh work.
func (s *State) recoverDistributionHistory() error {
	current, err := loadCurrent(s.config, s.storage)
	if err != nil {
		return err
	}
	s.current = current
	return s.reconcileDistributionJournal()
}

// reconcileDistributionJournal uses the distribution journal as the accepted
// floor. Only this mutable opening can repair a stale active pointer.
func (s *State) reconcileDistributionJournal() error {
	name, raw, err := s.storage.LoadControl()
	if err != nil {
		return fmt.Errorf("load distribution security state: %w", err)
	}
	if name == "" {
		if s.current != nil {
			return &RecoveryRequiredError{Reason: "distribution journal is missing from an active root"}
		}
		return nil
	}
	state, err := distribution.Decode(raw)
	if err != nil || distribution.Digest(raw) != name {
		return errors.New("distribution generation is invalid")
	}
	if state.EpochFloor != 0 && (s.current == nil || state.EpochFloor > s.current.Snapshot.Epoch ||
		state.EpochFloor == s.current.Snapshot.Epoch && state.EpochDigest != s.current.Snapshot.Digest) {
		if err := s.repairActiveMirror(state); err != nil {
			return err
		}
	}
	if s.current != nil && state.EpochFloor < s.current.Snapshot.Epoch {
		return errors.New("distribution security state is older than the active generation")
	}
	s.distribution = state
	return s.recoverPendingState()
}

func (s *State) repairActiveMirror(state distribution.Record) error {
	name := fmt.Sprintf("%x", state.EpochDigest)
	decision, err := loadStoredChain(s.config, s.storage, name)
	if err != nil {
		return fmt.Errorf("recover distribution active generation: %w", err)
	}
	if decision.Header.Number != state.EpochFloor || decision.Header.Digest != state.EpochDigest {
		return errors.New("distribution active identity disagrees with its generation")
	}
	// The floor repair must not activate a retired-schema generation (F-50).
	if err := classifyRetainedClosedSchema(s.config, decision.Header, "recovered active"); err != nil {
		return err
	}
	if err := persistDecision(s.storage, &decision, true); err != nil {
		return fmt.Errorf("repair active generation pointer: %w", err)
	}
	s.current = &decision
	return nil
}

func (s *State) recoverPendingState() error {
	state := s.distribution
	if state.PendingDigest == [32]byte{} {
		return nil
	}
	if s.current == nil {
		return errors.New("pending Epoch exists without an active predecessor")
	}
	if state.PendingDigest == s.current.Snapshot.Digest {
		state.PendingDigest = [32]byte{}
		state.PendingValidFrom = 0
		state.Sequence++
		return s.commitDistribution(state)
	}
	name := fmt.Sprintf("%x", state.PendingDigest)
	decision, err := loadNamedGeneration(s.config, s.storage, name, epochPredecessor(s.current))
	if err != nil {
		return fmt.Errorf("load pending Epoch: %w", err)
	}
	if decision.Header.ValidFrom.Unix() != state.PendingValidFrom {
		return errors.New("pending Epoch activation time disagrees with durable state")
	}
	// A retained old-schema pending generation must never become promotable
	// by a later Source wave (F-50).
	if err := classifyRetainedClosedSchema(s.config, decision.Header, "pending"); err != nil {
		return err
	}
	s.pendingDecision = &decision
	return nil
}

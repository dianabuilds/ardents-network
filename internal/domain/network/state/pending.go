package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// commitPendingDecision writes the authenticated generation before the
// control journal can name it. History reconciliation precedes this operation.
func (s *State) commitPendingDecision(decision *epoch.Decision, next distribution.Record) error {
	if decision == nil {
		// Preserve the historical zero-value staging refusal.
		decision = &epoch.Decision{}
	}
	newPending := s.pendingDecision == nil
	if newPending {
		if err := stageGeneration(s.storage, decision); err != nil {
			return err
		}
	}
	next.PendingDigest, next.PendingValidFrom = decision.Header.Digest, decision.Header.ValidFrom.Unix()
	if err := s.commitDistribution(next); err != nil {
		return err
	}
	if newPending {
		// The staged decision is immutable; callers never mutate it after
		// commit, so the pointer keeps the same owned facts.
		s.pendingDecision = decision
	}
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

package state

import (
	"errors"
	"fmt"
)

func (s *networkState) recoverPendingState() error {
	state := s.distribution
	if isZero32(state.pendingDigest) {
		return nil
	}
	if s.current == nil {
		return errors.New("pending Epoch exists without an active predecessor")
	}
	if state.pendingDigest == s.current.Snapshot.Digest {
		state.pendingDigest = [32]byte{}
		state.pendingValidFrom = 0
		state.sequence++
		return s.commitDistribution(state)
	}
	name := fmt.Sprintf("%x", state.pendingDigest)
	decision, err := loadNamedGeneration(s.config, s.storage, name, epochPredecessor(s.current))
	if err != nil {
		return fmt.Errorf("load pending Epoch: %w", err)
	}
	if decision.Header.ValidFrom.Unix() != state.pendingValidFrom {
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

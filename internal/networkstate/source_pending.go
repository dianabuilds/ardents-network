package networkstate

import (
	"errors"
	"fmt"
	"time"
)

func (s *store) recoverPendingState() error {
	state := s.distribution
	if isZero32(state.pendingDigest) {
		return nil
	}
	if s.current == nil {
		return errors.New("pending Epoch exists without an active predecessor")
	}
	if state.pendingDigest == s.current.Digest {
		state.pendingDigest = [32]byte{}
		state.pendingValidFrom = 0
		state.sequence++
		return s.commitDistribution(state)
	}
	name := fmt.Sprintf("%x", state.pendingDigest)
	decision, err := loadGeneration(s.config, name, s.current, false)
	if err != nil {
		return fmt.Errorf("load pending Epoch: %w", err)
	}
	if decision.epoch.validFrom.Unix() != state.pendingValidFrom {
		return errors.New("pending Epoch activation time disagrees with durable state")
	}
	s.pendingDecision = &decision
	return nil
}

func (s *store) activatePending(now time.Time) error {
	if s.pendingDecision == nil || now.Before(s.pendingDecision.epoch.validFrom) {
		return nil
	}
	if !now.Before(s.pendingDecision.epoch.validUntil) {
		return errors.New("pending Epoch expired before activation")
	}
	decision := *s.pendingDecision
	state := s.distribution
	state.sequence++
	state.epochFloor, state.epochDigest = decision.epoch.number, decision.epoch.digest
	state.trustedTimeFloor = max(state.trustedTimeFloor, now.Unix())
	state.pendingDigest, state.pendingValidFrom = [32]byte{}, 0
	if err := s.commitActiveDecision(decision, state); err != nil {
		return err
	}
	s.pendingDecision = nil
	return nil
}

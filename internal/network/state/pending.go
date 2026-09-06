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
	if state.pendingDigest == s.current.Digest {
		state.pendingDigest = [32]byte{}
		state.pendingValidFrom = 0
		state.sequence++
		return s.commitDistribution(state)
	}
	name := fmt.Sprintf("%x", state.pendingDigest)
	decision, err := loadNamedGeneration(s.config, s.storage, name, s.current)
	if err != nil {
		return fmt.Errorf("load pending Epoch: %w", err)
	}
	if decision.epoch.validFrom.Unix() != state.pendingValidFrom {
		return errors.New("pending Epoch activation time disagrees with durable state")
	}
	s.pendingDecision = &decision
	return nil
}

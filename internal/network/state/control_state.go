package state

import (
	"errors"
	"fmt"
)

type distributionState struct {
	sequence            uint64
	epochFloor          uint64
	epochDigest         [32]byte
	trustedTimeFloor    int64
	conflicting         bool
	consecutiveFailures uint64
	backoffLevel        byte
	nextAutomatic       int64
	history             [][32]byte
	cycleID             uint64
	cycleActive         bool
	cyclePurpose        byte
	cycleStarted        int64
	cycleDeadline       int64
	attempts            [4]byte
	outcomes            [4]byte
	requestedDigests    [2][32]byte
	observedEpochs      [4]uint64
	observedDigests     [4][32]byte
	pendingDigest       [32]byte
	pendingValidFrom    int64
	cycleSeed           [32]byte
	sourceOrder         [2]byte
}

func (s *store) loadDistributionState() error {
	name, raw, err := s.storage.LoadControl()
	if err != nil {
		return fmt.Errorf("load distribution security state: %w", err)
	}
	if name == "" {
		if s.current != nil {
			s.distribution.epochFloor = s.current.Epoch
			s.distribution.epochDigest = s.current.Digest
		}
		return nil
	}
	state, err := decodeDistributionState(raw)
	if err != nil || distributionDigest(raw) != name {
		return errors.New("distribution generation is invalid")
	}
	if state.epochFloor != 0 && (s.current == nil || state.epochFloor > s.current.Epoch ||
		state.epochFloor == s.current.Epoch && state.epochDigest != s.current.Digest) {
		if err := s.recoverDistributionActive(state); err != nil {
			return err
		}
	}
	if s.current != nil && state.epochFloor < s.current.Epoch {
		return errors.New("distribution security state is older than the active generation")
	}
	s.distribution = state
	return s.recoverPendingState()
}

func (s *store) recoverDistributionActive(state distributionState) error {
	name := fmt.Sprintf("%x", state.epochDigest)
	decision, err := loadStoredChain(s.config, s.storage, name)
	if err != nil {
		return fmt.Errorf("recover distribution active generation: %w", err)
	}
	if decision.epoch.number != state.epochFloor || decision.epoch.digest != state.epochDigest {
		return errors.New("distribution active identity disagrees with its generation")
	}
	if err := persistDecision(s.storage, decision, true); err != nil {
		return fmt.Errorf("repair active generation pointer: %w", err)
	}
	snapshot := decision.snapshot
	s.current, s.currentDecision = &snapshot, &decision
	return nil
}

func (s *store) commitDistribution(state distributionState) error {
	raw := encodeDistributionState(state)
	name := distributionDigest(raw)
	if err := s.storage.CommitControl(name, raw); err != nil {
		return err
	}
	s.distribution = state
	return nil
}

func (s *store) commitActiveDecision(decision candidateDecision, state distributionState) error {
	if err := persistDecision(s.storage, decision, false); err != nil {
		return err
	}
	state.epochFloor, state.epochDigest = decision.epoch.number, decision.epoch.digest
	if err := s.commitDistribution(state); err != nil {
		return err
	}
	snapshot := decision.snapshot
	s.current, s.currentDecision = &snapshot, &decision
	return persistDecision(s.storage, decision, true)
}

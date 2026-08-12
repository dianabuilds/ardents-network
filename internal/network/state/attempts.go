package state

import "errors"

func (s *store) beginLatestAttempt(index int) (bool, byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.distribution
	if !state.cycleActive || index < 0 || index > 1 {
		return false, 0, errors.New("LATEST source attempt is outside the active cycle")
	}
	if state.attempts[index] == 1 {
		state.sequence++
		state.attempts[index] = 3
		state.outcomes[index] = sourceOutcomeInterrupted
		if err := s.commitDistribution(state); err != nil {
			return false, 0, err
		}
		return false, sourceOutcomeInterrupted, nil
	}
	if state.attempts[index] != 0 {
		return false, state.outcomes[index], nil
	}
	state.sequence++
	state.attempts[index] = 1
	exposure := s.config.sourceInfo.Exposures[index]
	if !containsIdentity(state.history, exposure) {
		state.history = append(state.history, exposure)
	}
	if err := s.commitDistribution(state); err != nil {
		return false, 0, err
	}
	return true, 0, nil
}

func (s *store) beginDigestAttempt(source int, digest [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := 2 + source
	if !s.distribution.cycleActive || s.distribution.attempts[index] != 0 || isZero32(digest) {
		return errors.New("by-digest source attempt is not available")
	}
	state := s.distribution
	state.sequence++
	state.attempts[index] = 1
	state.requestedDigests[source] = digest
	exposure := s.config.sourceInfo.Exposures[source]
	if !containsIdentity(state.history, exposure) {
		state.history = append(state.history, exposure)
	}
	return s.commitDistribution(state)
}

func (s *store) finishDigestAttempt(source int, succeeded bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := 2 + source
	if !s.distribution.cycleActive || s.distribution.attempts[index] != 1 {
		return errors.New("by-digest source attempt is not started")
	}
	state := s.distribution
	state.sequence++
	state.attempts[index] = 3
	if succeeded {
		state.attempts[index] = 2
	}
	return s.commitDistribution(state)
}

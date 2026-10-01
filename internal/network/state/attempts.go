package state

import "errors"

// checkSourceExposureCapacity refuses a changed plan before its first contact
// could grow the durable history beyond the decoder's fixed bound. A validated
// Source plan contains two distinct exposure identities.
func (s *networkState) checkSourceExposureCapacity() error {
	count := len(s.distribution.history)
	for _, exposure := range s.config.sourceInfo.Exposures {
		if !containsIdentity(s.distribution.history, exposure) {
			count++
		}
	}
	if count > maximumSourceExposureHistory {
		return errors.New("direct Source exposure history is full")
	}
	return nil
}

func (s *networkState) beginLatestAttempt(index int) (bool, byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.distribution
	if !state.cycleActive || index < 0 || index > 1 {
		return false, 0, errors.New("LATEST source attempt is outside the active cycle")
	}
	if state.interruptUnresolvedAttempt(index) {
		state.sequence++
		if err := s.commitDistribution(state); err != nil {
			return false, 0, err
		}
		return false, sourceOutcomeInterrupted, nil
	}
	if state.attempts[index] != sourceAttemptNotStarted {
		return false, state.outcomes[index], nil
	}
	state.sequence++
	state.attempts[index] = sourceAttemptInFlight
	exposure := s.config.sourceInfo.Exposures[index]
	if !containsIdentity(state.history, exposure) {
		state.history = append(state.history, exposure)
	}
	if err := s.commitDistribution(state); err != nil {
		return false, 0, err
	}
	return true, 0, nil
}

func (s *networkState) beginDigestAttempt(source int, digest [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := digestAttemptSlot(source)
	if !s.distribution.cycleActive || s.distribution.attempts[index] != sourceAttemptNotStarted || digest == [32]byte{} {
		return errors.New("by-digest source attempt is not available")
	}
	state := s.distribution
	state.sequence++
	state.attempts[index] = sourceAttemptInFlight
	state.requestedDigests[source] = digest
	exposure := s.config.sourceInfo.Exposures[source]
	if !containsIdentity(state.history, exposure) {
		state.history = append(state.history, exposure)
	}
	return s.commitDistribution(state)
}

func (s *networkState) finishDigestAttempt(source int, responseCompleted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := digestAttemptSlot(source)
	if !s.distribution.cycleActive || s.distribution.attempts[index] != sourceAttemptInFlight {
		return errors.New("by-digest source attempt is not started")
	}
	state := s.distribution
	state.sequence++
	state.attempts[index] = sourceAttemptFailed
	if responseCompleted {
		state.attempts[index] = sourceAttemptCompleted
	}
	return s.commitDistribution(state)
}

// interruptUnresolvedAttempt closes an attempt whose result was not recorded.
// A BY_DIGEST response can be marked completed before its object is verified;
// after a crash its missing outcome must not look like "not attempted".
func (state *distributionState) interruptUnresolvedAttempt(slot int) bool {
	status := state.attempts[slot]
	if status != sourceAttemptInFlight &&
		(status != sourceAttemptCompleted || state.outcomes[slot] != 0) {
		return false
	}
	state.attempts[slot] = sourceAttemptFailed
	state.outcomes[slot] = sourceOutcomeInterrupted
	return true
}

func containsIdentity(history [][32]byte, identity [32]byte) bool {
	for _, current := range history {
		if current == identity {
			return true
		}
	}
	return false
}

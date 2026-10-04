package state

import "errors"

// checkSourceExposureCapacity refuses a changed plan before its first contact
// could grow the durable history beyond the decoder's fixed bound. A validated
// Source plan contains two distinct exposure identities.
func (s *networkState) checkSourceExposureCapacity() error {
	return sourceExposureCapacity(s.distribution, s.config.sourceInfo.Exposures)
}

func (s *networkState) beginLatestAttempt(index int) (bool, byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index > 1 {
		return false, 0, errors.New("LATEST source attempt is outside the active cycle")
	}
	state, contact, outcome, err := proposeLatestAttempt(s.distribution, index, s.config.sourceInfo.Exposures[index])
	if err != nil {
		return false, 0, err
	}
	if state.sequence != s.distribution.sequence {
		if err := s.commitDistribution(state); err != nil {
			return false, 0, err
		}
	}
	return contact, outcome, nil
}

func (s *networkState) beginDigestAttempt(source int, digest [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if source < 0 || source > 1 {
		return errors.New("by-digest source attempt is not available")
	}
	state, err := proposeDigestAttempt(s.distribution, source, digest, s.config.sourceInfo.Exposures[source])
	if err != nil {
		return err
	}
	return s.commitDistribution(state)
}

func (s *networkState) finishDigestAttempt(source int, responseCompleted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := proposeDigestCompletion(s.distribution, source, responseCompleted)
	if err != nil {
		return err
	}
	return s.commitDistribution(state)
}

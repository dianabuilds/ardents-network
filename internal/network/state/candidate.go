package state

import (
	"bytes"
	"errors"
)

func (s *networkState) verifySourceBundle(bundle sourceBundle, current *Snapshot, currentDecision *candidateDecision) (candidateDecision, error) {
	if len(bundle.materials) != 1 {
		return candidateDecision{}, errors.New("source withheld the requested materialization index")
	}
	index, err := materializationIndex(bundle.materials[0])
	if err != nil || index != s.config.sourceInfo.MaterialIndex {
		return candidateDecision{}, errors.New("source withheld the requested materialization index")
	}
	parsed, err := parseEpoch(bundle.epoch)
	if err != nil {
		return candidateDecision{}, err
	}
	verification := s.config
	verification.now = verification.clock().UTC()
	if verification.now.Before(parsed.validFrom) {
		verification.now = parsed.validFrom
	}
	if current != nil && parsed.number == current.Epoch && parsed.digest == current.Digest {
		if currentDecision == nil || !bytes.Equal(bundle.epoch, currentDecision.epochBytes) || !equalInputs(bundle.inputs, currentDecision.inputs) {
			return candidateDecision{}, errors.New("source changed bytes for the current Epoch")
		}
		if err := verifyDecisionMaterials(*currentDecision, bundle.materials); err != nil {
			return candidateDecision{}, err
		}
		return *currentDecision, nil
	}
	if s.pendingDecision != nil && parsed.number == s.pendingDecision.epoch.number && parsed.digest == s.pendingDecision.epoch.digest {
		if !bytes.Equal(bundle.epoch, s.pendingDecision.epochBytes) || !equalInputs(bundle.inputs, s.pendingDecision.inputs) {
			return candidateDecision{}, errors.New("source changed bytes for the pending Epoch")
		}
		if err := verifyDecisionMaterials(*s.pendingDecision, bundle.materials); err != nil {
			return candidateDecision{}, err
		}
		return *s.pendingDecision, nil
	}
	return verifyDecision(verification, current, bundle.epoch, bundle.inputs, bundle.materials, true)
}

func equalInputs(first, second [][]byte) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if !bytes.Equal(first[index], second[index]) {
			return false
		}
	}
	return true
}

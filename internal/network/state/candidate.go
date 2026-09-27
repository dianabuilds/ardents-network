package state

import (
	"bytes"
	"errors"
)

func (s *networkState) verifySourceBundle(bundle sourceBundle, current *Snapshot, currentDecision *verifiedEpochDecision) (verifiedEpochDecision, error) {
	if len(bundle.materials) != 1 {
		return verifiedEpochDecision{}, errors.New("source withheld the requested materialization index")
	}
	index, err := materializationIndex(bundle.materials[0])
	if err != nil || index != s.config.sourceInfo.MaterialIndex {
		return verifiedEpochDecision{}, errors.New("source withheld the requested materialization index")
	}
	parsed, err := parseEpoch(bundle.epoch)
	if err != nil {
		return verifiedEpochDecision{}, err
	}
	verification := s.config
	verification.now = verification.clock().UTC()
	if verification.now.Before(parsed.validFrom) {
		verification.now = parsed.validFrom
	}
	if current != nil && parsed.number == current.Epoch && parsed.digest == current.Digest {
		if currentDecision == nil || !bytes.Equal(bundle.epoch, currentDecision.EpochBytes) || !equalInputs(bundle.inputs, currentDecision.Inputs) {
			return verifiedEpochDecision{}, errors.New("source changed bytes for the current Epoch")
		}
		if err := verifyDecisionMaterials(*currentDecision, bundle.materials); err != nil {
			return verifiedEpochDecision{}, err
		}
		return *currentDecision, nil
	}
	if s.pendingDecision != nil && parsed.number == s.pendingDecision.epoch.number && parsed.digest == s.pendingDecision.epoch.digest {
		if !verification.now.Before(s.pendingDecision.epoch.validUntil) {
			return verifiedEpochDecision{}, errors.New("pending Epoch is not strictly current")
		}
		if !bytes.Equal(bundle.epoch, s.pendingDecision.EpochBytes) || !equalInputs(bundle.inputs, s.pendingDecision.Inputs) {
			return verifiedEpochDecision{}, errors.New("source changed bytes for the pending Epoch")
		}
		if err := verifyDecisionMaterials(*s.pendingDecision, bundle.materials); err != nil {
			return verifiedEpochDecision{}, err
		}
		return *s.pendingDecision, nil
	}
	decision, err := verifyDecision(verification, epochPredecessor(current), bundle.epoch, bundle.inputs, bundle.materials, true)
	if err != nil {
		return verifiedEpochDecision{}, err
	}
	// A Source wave can only stage or activate a candidate that passed the
	// sole closed intake schema; the reuse branches above return retained
	// decisions that Open already classified (F-50).
	if err := requireClosedIntakeSchema(s.config, decision.epoch); err != nil {
		return verifiedEpochDecision{}, err
	}
	return decision, nil
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

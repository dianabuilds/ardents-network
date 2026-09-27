package state

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

func (s *networkState) verifySourceBundle(bundle sourceBundle, current *Snapshot, currentDecision *epoch.Decision) (epoch.Decision, error) {
	if len(bundle.materials) != 1 {
		return epoch.Decision{}, errors.New("source withheld the requested materialization index")
	}
	index, err := materializationIndex(bundle.materials[0])
	if err != nil || index != s.config.sourceInfo.MaterialIndex {
		return epoch.Decision{}, errors.New("source withheld the requested materialization index")
	}
	parsed, err := epoch.Inspect(bundle.epoch)
	if err != nil {
		return epoch.Decision{}, err
	}
	verification := s.config
	verification.now = verification.clock().UTC()
	if verification.now.Before(parsed.ValidFrom) {
		verification.now = parsed.ValidFrom
	}
	if current != nil && parsed.Number == current.Epoch && parsed.Digest == current.Digest {
		if currentDecision == nil || !bytes.Equal(bundle.epoch, currentDecision.EpochBytes) || !equalInputs(bundle.inputs, currentDecision.Inputs) {
			return epoch.Decision{}, errors.New("source changed bytes for the current Epoch")
		}
		if err := verifyDecisionMaterials(*currentDecision, bundle.materials); err != nil {
			return epoch.Decision{}, err
		}
		return *currentDecision, nil
	}
	if s.pendingDecision != nil && parsed.Number == s.pendingDecision.Header.Number && parsed.Digest == s.pendingDecision.Header.Digest {
		if !verification.now.Before(s.pendingDecision.Header.ValidUntil) {
			return epoch.Decision{}, errors.New("pending Epoch is not strictly current")
		}
		if !bytes.Equal(bundle.epoch, s.pendingDecision.EpochBytes) || !equalInputs(bundle.inputs, s.pendingDecision.Inputs) {
			return epoch.Decision{}, errors.New("source changed bytes for the pending Epoch")
		}
		if err := verifyDecisionMaterials(*s.pendingDecision, bundle.materials); err != nil {
			return epoch.Decision{}, err
		}
		return *s.pendingDecision, nil
	}
	decision, err := verifyDecision(verification, epochPredecessor(current), bundle.epoch, bundle.inputs, bundle.materials, true)
	if err != nil {
		return epoch.Decision{}, err
	}
	// A Source wave can only stage or activate a candidate that passed the
	// sole closed intake schema; the reuse branches above return retained
	// decisions that Open already classified (F-50).
	if err := requireClosedIntakeSchema(s.config, decision.Header); err != nil {
		return epoch.Decision{}, err
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

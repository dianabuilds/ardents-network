package state

import (
	"bytes"
	"errors"

	epoch2 "github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func (s *networkState) verifySourceBundle(bundle source.Bundle, current *epoch2.Decision) (epoch2.Decision, error) {
	if len(bundle.Materials) != 1 {
		return epoch2.Decision{}, errors.New("source withheld the requested materialization index")
	}
	index, err := epoch2.InspectMaterializationIndex(bundle.Materials[0])
	if err != nil || index != s.config.sourceInfo.MaterialIndex {
		return epoch2.Decision{}, errors.New("source withheld the requested materialization index")
	}
	parsed, err := epoch2.Inspect(bundle.Epoch)
	if err != nil {
		return epoch2.Decision{}, err
	}
	verification := s.config
	verification.now = verification.clock().UTC()
	if verification.now.Before(parsed.ValidFrom) {
		verification.now = parsed.ValidFrom
	}
	if current != nil && parsed.Number == current.Snapshot.Epoch && parsed.Digest == current.Snapshot.Digest {
		if !bytes.Equal(bundle.Epoch, current.EpochBytes) || !equalInputs(bundle.Inputs, current.Inputs) {
			return epoch2.Decision{}, errors.New("source changed bytes for the current Epoch")
		}
		if err := verifyDecisionMaterials(*current, bundle.Materials); err != nil {
			return epoch2.Decision{}, err
		}
		return *current, nil
	}
	if s.pendingDecision != nil && parsed.Number == s.pendingDecision.Header.Number && parsed.Digest == s.pendingDecision.Header.Digest {
		if !verification.now.Before(s.pendingDecision.Header.ValidUntil) {
			return epoch2.Decision{}, errors.New("pending Epoch is not strictly current")
		}
		if !bytes.Equal(bundle.Epoch, s.pendingDecision.EpochBytes) || !equalInputs(bundle.Inputs, s.pendingDecision.Inputs) {
			return epoch2.Decision{}, errors.New("source changed bytes for the pending Epoch")
		}
		if err := verifyDecisionMaterials(*s.pendingDecision, bundle.Materials); err != nil {
			return epoch2.Decision{}, err
		}
		return *s.pendingDecision, nil
	}
	// Authenticate independently of successor admissibility: a signed second
	// digest for the retained number must reach history reconciliation even
	// when another Source supplies an otherwise valid successor.
	decision, err := authenticateDecision(verification, bundle.Epoch, bundle.Inputs, bundle.Materials, true)
	if err != nil {
		return epoch2.Decision{}, err
	}
	// A Source wave can only stage or activate a candidate that passed the
	// sole closed intake schema; the reuse branches above return retained
	// decisions that Open already classified (F-50).
	if err := requireClosedIntakeSchema(s.config, decision.Header); err != nil {
		return epoch2.Decision{}, err
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

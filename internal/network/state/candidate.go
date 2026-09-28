package state

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func (s *networkState) verifySourceBundle(bundle source.Bundle, current *epoch.Decision) (epoch.Decision, error) {
	if len(bundle.Materials) != 1 {
		return epoch.Decision{}, errors.New("source withheld the requested materialization index")
	}
	index, err := materializationIndex(bundle.Materials[0])
	if err != nil || index != s.config.sourceInfo.MaterialIndex {
		return epoch.Decision{}, errors.New("source withheld the requested materialization index")
	}
	parsed, err := epoch.Inspect(bundle.Epoch)
	if err != nil {
		return epoch.Decision{}, err
	}
	verification := s.config
	verification.now = verification.clock().UTC()
	if verification.now.Before(parsed.ValidFrom) {
		verification.now = parsed.ValidFrom
	}
	if current != nil && parsed.Number == current.Snapshot.Epoch && parsed.Digest == current.Snapshot.Digest {
		if !bytes.Equal(bundle.Epoch, current.EpochBytes) || !equalInputs(bundle.Inputs, current.Inputs) {
			return epoch.Decision{}, errors.New("source changed bytes for the current Epoch")
		}
		if err := verifyDecisionMaterials(*current, bundle.Materials); err != nil {
			return epoch.Decision{}, err
		}
		return *current, nil
	}
	if s.pendingDecision != nil && parsed.Number == s.pendingDecision.Header.Number && parsed.Digest == s.pendingDecision.Header.Digest {
		if !verification.now.Before(s.pendingDecision.Header.ValidUntil) {
			return epoch.Decision{}, errors.New("pending Epoch is not strictly current")
		}
		if !bytes.Equal(bundle.Epoch, s.pendingDecision.EpochBytes) || !equalInputs(bundle.Inputs, s.pendingDecision.Inputs) {
			return epoch.Decision{}, errors.New("source changed bytes for the pending Epoch")
		}
		if err := verifyDecisionMaterials(*s.pendingDecision, bundle.Materials); err != nil {
			return epoch.Decision{}, err
		}
		return *s.pendingDecision, nil
	}
	decision, err := verifyDecision(verification, epochPredecessor(current), bundle.Epoch, bundle.Inputs, bundle.Materials, true)
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

func materializationIndex(raw []byte) (uint32, error) {
	if len(raw) < 36 || len(raw) > epoch.MaxMaterializationBytes {
		return 0, errors.New("materialization framing length is invalid")
	}
	return binary.BigEndian.Uint32(raw[32:36]), nil
}

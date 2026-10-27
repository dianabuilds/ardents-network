package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// readRetainedInspection verifies signed chain and security/pending floors only.
// Each independently instantiated semantic owner brackets these reads with its
// own native/participation checks and samples trusted time after physical I/O.
// It never opens a mutable State, recovers pointers or publishes runtime rights.
func readRetainedInspection(cfg config, storage *durable.Inspection) (*epoch.Decision, distributionState, error) {
	name, values, err := storage.ReadState()
	if err != nil {
		return nil, distributionState{}, err
	}
	current, err := loadCurrentValues(cfg, name, values)
	if err != nil {
		return nil, distributionState{}, err
	}
	if current == nil {
		return nil, distributionState{}, ErrNoCurrentGeneration
	}
	control, raw, err := storage.ReadControl()
	if err != nil {
		return nil, distributionState{}, err
	}
	journal, err := decodeDistributionState(raw)
	if err != nil || control == "" || distributionDigest(raw) != control || journal.epochFloor != current.Header.Number || journal.epochDigest != current.Header.Digest || journal.conflicting || journal.cycleActive {
		return nil, distributionState{}, errors.Join(errors.New("retained State security history requires explicit recovery"), err)
	}
	if journal.pendingDigest == [32]byte{} {
		return current, journal, nil
	}
	if journal.pendingDigest == current.Header.Digest {
		return nil, distributionState{}, errors.New("retained State pending current requires explicit recovery")
	}
	for _, value := range values {
		if value.Name != fmt.Sprintf("%x", journal.pendingDigest) {
			continue
		}
		decision, err := loadGeneration(cfg, value, epochPredecessor(current))
		if err != nil || decision.Header.ValidFrom.Unix() != journal.pendingValidFrom {
			return nil, distributionState{}, errors.Join(errors.New("retained State pending history differs"), err)
		}
		if err := classifyRetainedClosedSchema(cfg, decision.Header, "inspection pending"); err != nil {
			return nil, distributionState{}, err
		}
		return current, journal, nil
	}
	return nil, distributionState{}, errors.New("retained State pending generation missing")
}

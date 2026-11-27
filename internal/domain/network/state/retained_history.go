package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

type retainedStateReader interface {
	ReadState() (string, []durable.Generation, error)
	ReadControl() (string, []byte, error)
}

// readRetainedHistory verifies the signed chain and security/pending floors.
// Each independently instantiated semantic owner brackets these reads with its
// own native/participation checks and samples trusted time after physical I/O.
// It never opens a mutable State, recovers pointers or publishes runtime rights.
func readRetainedHistory(cfg config, storage retainedStateReader) (*epoch.Decision, distribution.Record, error) {
	name, values, err := storage.ReadState()
	if err != nil {
		return nil, distribution.Record{}, err
	}
	current, err := loadCurrentValues(cfg, name, values)
	if err != nil {
		return nil, distribution.Record{}, err
	}
	if current == nil {
		return nil, distribution.Record{}, ErrNoCurrentGeneration
	}
	control, raw, err := storage.ReadControl()
	if err != nil {
		return nil, distribution.Record{}, err
	}
	journal, err := distribution.Decode(raw)
	if err != nil || control == "" || distribution.Digest(raw) != control || journal.EpochFloor != current.Header.Number || journal.EpochDigest != current.Header.Digest || journal.Conflicting || journal.CycleActive {
		return nil, distribution.Record{}, errors.Join(errors.New("retained State security history requires explicit recovery"), err)
	}
	if journal.PendingDigest == [32]byte{} {
		return current, journal, nil
	}
	if journal.PendingDigest == current.Header.Digest {
		return nil, distribution.Record{}, errors.New("retained State pending current requires explicit recovery")
	}
	for _, value := range values {
		if value.Name != fmt.Sprintf("%x", journal.PendingDigest) {
			continue
		}
		decision, err := loadGeneration(cfg, value, epochPredecessor(current))
		if err != nil || decision.Header.ValidFrom.Unix() != journal.PendingValidFrom {
			return nil, distribution.Record{}, errors.Join(errors.New("retained State pending history differs"), err)
		}
		if err := classifyRetainedClosedSchema(cfg, decision.Header, "inspection pending"); err != nil {
			return nil, distribution.Record{}, err
		}
		return current, journal, nil
	}
	return nil, distribution.Record{}, errors.New("retained State pending generation missing")
}

package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/journal"
)

type retainedStateReader interface {
	ReadState() (string, []durable.Generation, error)
	ReadControl() (string, []byte, error)
}

// readRetainedHistory verifies the signed chain and security/pending floors.
// Each independently instantiated semantic owner brackets these reads with its
// own native/participation checks and samples trusted time after physical I/O.
// It never opens a mutable State, recovers pointers or publishes runtime rights.
func readRetainedHistory(cfg config, storage retainedStateReader) (*epoch.Decision, journal.Record, error) {
	name, values, err := storage.ReadState()
	if err != nil {
		return nil, journal.Record{}, err
	}
	current, err := loadCurrentValues(cfg, name, values)
	if err != nil {
		return nil, journal.Record{}, err
	}
	if current == nil {
		return nil, journal.Record{}, ErrNoCurrentGeneration
	}
	control, raw, err := storage.ReadControl()
	if err != nil {
		return nil, journal.Record{}, err
	}
	record, err := journal.Decode(raw)
	if err != nil || control == "" || journal.Digest(raw) != control || record.EpochFloor != current.Header.Number || record.EpochDigest != current.Header.Digest || record.Conflicting || record.CycleActive {
		return nil, journal.Record{}, errors.Join(errors.New("retained State security history requires explicit recovery"), err)
	}
	if record.PendingDigest == [32]byte{} {
		return current, record, nil
	}
	if record.PendingDigest == current.Header.Digest {
		return nil, journal.Record{}, errors.New("retained State pending current requires explicit recovery")
	}
	for _, value := range values {
		if value.Name != fmt.Sprintf("%x", record.PendingDigest) {
			continue
		}
		decision, err := loadGeneration(cfg, value, epochPredecessor(current))
		if err != nil || decision.Header.ValidFrom.Unix() != record.PendingValidFrom {
			return nil, journal.Record{}, errors.Join(errors.New("retained State pending history differs"), err)
		}
		if err := classifyRetainedClosedSchema(cfg, decision.Header, "inspection pending"); err != nil {
			return nil, journal.Record{}, err
		}
		return current, record, nil
	}
	return nil, journal.Record{}, errors.New("retained State pending generation missing")
}

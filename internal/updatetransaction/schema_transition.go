package updatetransaction

import (
	"context"
	"crypto/sha256"
	"errors"
)

type schemaTransition struct {
	work      SchemaWork
	copy      bool
	prepared  bool
	preserved SchemaSelection
	candidate SchemaSelection
	record    []byte
}

func planSchema(ctx context.Context, request Request, inspection rootInspection) (schemaTransition, error) {
	if request.SchemaPlan == "no-op-v1" {
		if inspection.schemaCurrent == nil {
			return schemaTransition{}, nil
		}
		if request.Schema == nil {
			return schemaTransition{}, errRecordInvalid
		}
		return schemaTransition{work: request.Schema, preserved: inspection.schemaCurrent.Selection,
			candidate: inspection.schemaCurrent.Selection}, nil
	}
	if request.SchemaPlan != "copy-on-write-v1" || request.Schema == nil || inspection.schemaCurrent == nil {
		return schemaTransition{}, errRecordInvalid
	}
	candidate, admitted, err := request.Schema.Plan(ctx, request.Generation, request.SchemaPlan, inspection.schemaCurrent.Selection)
	if err != nil {
		return schemaTransition{}, err
	}
	if !admitted || !validSchemaSuccessor(inspection.schemaCurrent.Selection, candidate) {
		return schemaTransition{}, errResourceDenied
	}
	record, err := encodeSchemaCurrent(schemaCurrent{Transaction: request.Generation, Selection: candidate,
		Predecessor: sha256.Sum256(inspection.schemaRaw)})
	if err != nil {
		return schemaTransition{}, err
	}
	return schemaTransition{work: request.Schema, copy: true, preserved: inspection.schemaCurrent.Selection,
		candidate: candidate, record: record}, nil
}

func validSchemaSuccessor(previous, candidate SchemaSelection) bool {
	if previous.Owner == ([32]byte{}) || candidate.Owner != previous.Owner || candidate.Generation != previous.Generation+1 ||
		candidate.Content == ([32]byte{}) || candidate.Bytes > maximumSchemaBytes || candidate.Entries > maximumSchemaEntries {
		return false
	}
	return candidate.Identity == schemaSelectionIdentity(candidate)
}

func (transition *schemaTransition) prepare(ctx context.Context) error {
	if transition.work == nil {
		return nil
	}
	if transition.copy {
		// A failed Prepare may have created foreign residue. Mark it as owned
		// cleanup input before the call so every failure invokes Discard.
		transition.prepared = true
		if err := transition.work.Prepare(ctx, transition.candidate); err != nil {
			return err
		}
	}
	return transition.work.Inspect(ctx, transition.candidate)
}

func (transition *schemaTransition) discard(ctx context.Context) error {
	if transition == nil || !transition.copy || !transition.prepared {
		return nil
	}
	err := transition.work.Discard(ctx, transition.candidate)
	if err == nil {
		transition.prepared = false
	}
	return err
}

func (transition schemaTransition) resourceParts() [][]byte {
	if !transition.copy {
		return nil
	}
	// The immutable successor record and its atomic sibling temp are both
	// included in admission before foreign schema materialization begins.
	return [][]byte{transition.record, transition.record}
}

func (transition schemaTransition) commit(store *ownedStore) error {
	if !transition.copy {
		return nil
	}
	if !transition.prepared {
		return errors.New("schema candidate was not prepared")
	}
	return store.commitSchema(transition.record)
}

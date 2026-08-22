package updatetransaction

import (
	"crypto/sha256"
	"fmt"
)

// validateSchemaSelection checks the transaction-owned selection fact only.
// It deliberately does not reopen the Adapter-owned schema root during
// recovery; a later Apply is responsible for that bounded foreign inspection.
func validateSchemaSelection(facts inventoryResult, selection currentSelection) error {
	if len(facts.SchemaCurrent.Bytes) == 0 {
		return nil
	}
	schema, err := decodeSchemaCurrent(facts.SchemaCurrent.Bytes)
	if err != nil {
		return fmt.Errorf("%w: schema selection: %v", errPlanInvalid, err)
	}
	matchesCode := schema.Transaction == selection.Transaction
	if selection.Rollback != nil {
		matchesCode = matchesCode || schema.Transaction == selection.Rollback.Generation
	}
	if !matchesCode {
		return fmt.Errorf("%w: schema/code selection mismatch", errPlanInvalid)
	}
	return nil
}

func verifyGenerationMatchesTuple(facts inventoryResult, tuple inspectedTuple) error {
	generation := generationByID(facts.Generations, tuple.Generation)
	if generation == nil || uint64(len(generation.Artifact.Bytes)) != tuple.Length ||
		sha256.Sum256(generation.Artifact.Bytes) != tuple.Artifact || sha256.Sum256(generation.Manifest.Bytes) != tuple.Manifest {
		return fmt.Errorf("%w: selected generation %d mismatch", errPlanInvalid, tuple.Generation)
	}
	return nil
}

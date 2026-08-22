package update

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
	if len(facts.SchemaTemps) == 1 {
		if selection.Rollback == nil || schema.Transaction != selection.Rollback.Generation {
			return fmt.Errorf("%w: schema temp without predecessor selection", errPlanInvalid)
		}
		temporary, tempErr := decodeSchemaCurrent(facts.SchemaTemps[0].Bytes)
		if tempErr != nil || temporary.Transaction != selection.Transaction ||
			temporary.Predecessor != sha256.Sum256(facts.SchemaCurrent.Bytes) {
			return fmt.Errorf("%w: schema temp is not bound", errPlanInvalid)
		}
	}
	return nil
}

// applySchemaTempRecovery removes only an exact unselected selection temp.
// Recover never infers a commit from a temp; a later Apply re-inspects the
// Adapter-owned candidate before selecting it.
func applySchemaTempRecovery(plan recoveryPlan, facts inventoryResult, validation journalValidation) (recoveryPlan, error) {
	if len(facts.SchemaTemps) == 0 {
		return plan, nil
	}
	if len(validation.Entries) != int(stateSelfTesting) ||
		validation.Entries[stateSelfTesting-1].AdapterResult != adapterSuccess || plan.State != "self-testing" {
		return recoveryPlan{}, fmt.Errorf("%w: schema temp at incompatible state", errPlanInvalid)
	}
	plan.Operations = append(plan.Operations,
		planOperation{Kind: opRemoveFile, Path: facts.SchemaTemps[0].Name},
		planOperation{Kind: opSyncDirectory, Path: "."})
	return plan, nil
}

func verifyGenerationMatchesTuple(facts inventoryResult, tuple inspectedTuple) error {
	generation := generationByID(facts.Generations, tuple.Generation)
	if generation == nil || uint64(len(generation.Artifact.Bytes)) != tuple.Length ||
		sha256.Sum256(generation.Artifact.Bytes) != tuple.Artifact || sha256.Sum256(generation.Manifest.Bytes) != tuple.Manifest {
		return fmt.Errorf("%w: selected generation %d mismatch", errPlanInvalid, tuple.Generation)
	}
	return nil
}

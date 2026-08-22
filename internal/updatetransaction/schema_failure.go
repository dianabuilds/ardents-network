package updatetransaction

import (
	"context"
	"errors"
)

// schemaPreparationFailure runs before code activation. It therefore removes
// only the Adapter-owned unselected candidate and the transaction's own
// prefix, leaving the old code and schema selections intact.
func schemaPreparationFailure(store *ownedStore, request Request, inspection rootInspection, schema *schemaTransition, cause error) (Result, error) {
	discardErr := schema.discard(context.Background())
	cleanupErr := store.cleanup(request.Generation)
	releaseErr := store.release()
	if discardErr != nil || cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: "draining", Generation: request.Generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, discardErr, cleanupErr, releaseErr)
	}
	result := stagingFailureResult(request, "draining")
	result.CurrentDigest = inspection.selection.Current.Artifact
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	result.CustodyNotice = inspection.currentCustody
	return result, errors.Join(cause, releaseErr)
}

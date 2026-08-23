package update

import "errors"

// activationBusyFailure preserves the state-six recovery evidence after a
// platform contention refusal. It neither retries replacement nor removes a
// generation that may have been published before Windows rejected current.
func activationBusyFailure(store *ownedStore, request Request, inspection rootInspection, cause error) (Result, error) {
	result := Result{Outcome: "resource-denied", State: "busy", Generation: request.generation,
		CurrentDigest: inspection.selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update activation busy"}
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	return result, errors.Join(cause, store.release())
}

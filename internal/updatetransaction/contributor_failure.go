package updatetransaction

import (
	"context"
	"errors"
)

// rejoinFailure has the same safety meaning as a local self-test failure:
// candidate code remains rollback-pending and normal networking stays stopped.
func rejoinFailure(ctx context.Context, trace *tracer, store *ownedStore, request Request, inspection rootInspection, artifact [32]byte, cause error) (Result, error) {
	recordErr := trace.record(ctx, "08-self-testing", stateSelfTesting, adapterFailed)
	if recordErr != nil {
		return applyFailure(store, request, "self-testing", false, errors.Join(cause, recordErr))
	}
	pendingErr := trace.record(context.Background(), "10-rollback-pending", stateRollbackPending, adapterNotCalled)
	if pendingErr != nil {
		return applyFailure(store, request, "self-testing", false, errors.Join(cause, pendingErr))
	}
	result := selfTestFailedResult(request.Generation, artifact, inspection.selection.Current.Artifact, inspection.currentCustody)
	return result, errors.Join(cause, store.release())
}

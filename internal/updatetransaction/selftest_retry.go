package updatetransaction

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"
)

// resumeUnavailableSelfTest admits only the immutable state-8 unavailable
// prefix. It neither stages nor drains again: the previously selected
// candidate is rechecked under this invocation's bounded context.
func resumeUnavailableSelfTest(ctx context.Context, store *ownedStore, inspection rootInspection, request Request,
	artifact, manifest [32]byte, start, callerLimit time.Time) (Result, bool, error) {
	selection := inspection.selection
	if selection.Transaction != request.Generation {
		return Result{}, false, nil
	}
	if selection.Rollback == nil || selection.Current.Artifact != artifact || selection.Current.Manifest != manifest {
		return invalidResult(request, "committed"), true, errors.Join(errRecordInvalid, store.release())
	}
	facts, err := collectInventory(request.UpdateRoot)
	if err != nil || facts.InterruptedSelection != request.Generation {
		return transactionInvalidResult(request.Generation), true, errors.Join(err, store.release())
	}
	records, err := reconstructRecoveryRecords(facts)
	if err != nil {
		return transactionInvalidResult(request.Generation), true, errors.Join(err, store.release())
	}
	candidateArtifact, candidateManifest := candidateCommitments(facts, request.Generation)
	validation, err := validateJournal(request.Generation, facts.journalLookup(request.Generation), records.predecessorCommitment,
		candidateArtifact, candidateManifest)
	if err != nil || len(validation.Entries) != int(stateSelfTesting) ||
		validation.Entries[stateSelfTesting-1].AdapterResult != adapterUnavailable {
		return transactionInvalidResult(request.Generation), true, errors.Join(errRecordInvalid, err, store.release())
	}
	trace := &tracer{store: store, request: request, start: start, artifact: artifact, manifest: manifest,
		predecessor: sha256.Sum256(validation.RawEntries[stateSelfTesting-1]), callerLimit: callerLimit}
	identity := CandidateIdentity{Generation: request.Generation, TargetPath: request.Decision.Path, Length: request.Decision.Length,
		Digest: artifact, Platform: request.Decision.Platform, Architecture: request.Decision.Architecture,
		Environment: request.Decision.Environment, Network: request.Decision.Network}
	if callErr := callBounded(ctx, trace.deadline(stateSelfTesting), func(callCtx context.Context) error {
		return request.SelfTest.Check(callCtx, identity)
	}); callErr != nil {
		if selfTestUnavailableOnly(callErr) {
			return networkingUnverifiedResult(request.Generation, artifact, selection.Rollback.Artifact, inspection.currentCustody), true,
				errors.Join(callErr, store.release())
		}
		result, applyErr := applyFailure(store, request, "self-testing", false, callErr)
		return result, true, applyErr
	}
	if err := trace.record(ctx, "09-committed", stateCommitted, adapterSuccess); err != nil {
		result, applyErr := applyFailure(store, request, "committed", false, err)
		return result, true, applyErr
	}
	result := committedResult(request.Generation, artifact, selection.Rollback.Artifact, "update committed", inspection.currentCustody)
	return result, true, store.release()
}

func networkingUnverifiedResult(generation uint64, current, rollback [32]byte, custody string) Result {
	return Result{Outcome: "application-networking-unverified", State: "self-testing", Generation: generation,
		CurrentDigest: current, RollbackDigest: rollback, StagingPresent: false,
		SafeNotice: "update networking unverified", CustodyNotice: custody}
}

package updatetransaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

var errRollbackPending = errors.New("update rollback remains pending")
var errRollbackRefused = errors.New("update rollback decision is refused")

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
	if err != nil {
		return transactionInvalidResult(request.Generation), true, errors.Join(errRecordInvalid, err, store.release())
	}
	if len(validation.Entries) != int(stateSelfTesting) || validation.Entries[stateSelfTesting-1].AdapterResult != adapterUnavailable {
		return Result{}, false, nil
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

func selfTestFailedResult(generation uint64, current, rollback [32]byte, custody string) Result {
	return Result{Outcome: "self-test-failed", State: "rollback-pending", Generation: generation,
		CurrentDigest: current, RollbackDigest: rollback, StagingPresent: false,
		SafeNotice: "update self-test failed", CustodyNotice: custody}
}

// resumeRollbackPending recognizes the durable pre-rollback state and never
// repeats admission, drain, activation, or the failed self-test. A later slice
// may proceed only after validating Request.RollbackDecision against the exact
// retained manifest and payload under this same owned-root lock.
func resumeRollbackPending(store *ownedStore, inspection rootInspection, request Request,
	artifact, manifest [32]byte) (Result, bool, error) {
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
	if err != nil || len(validation.Entries) != 9 || validation.Entries[8].State != stateRollbackPending {
		return transactionInvalidResult(request.Generation), true, errors.Join(errRecordInvalid, err, store.release())
	}
	if request.RollbackDecision.Outcome != "" {
		if rollbackErr := validateRollbackDecision(store, request, selection); rollbackErr != nil {
			trace := &tracer{store: store, request: request, start: time.Now(), artifact: artifact, manifest: manifest,
				predecessor: sha256.Sum256(validation.RawEntries[8]), elapsedOffset: validation.Entries[8].ElapsedNanos}
			if recordErr := trace.record(context.Background(), "12-repair-required", stateRepairRequired, adapterNotCalled); recordErr != nil {
				return transactionInvalidResult(request.Generation), true, errors.Join(rollbackErr, recordErr, store.release())
			}
			return rollbackRefusedResult(request.Generation, artifact, selection.Rollback.Artifact, inspection.currentCustody), true,
				errors.Join(rollbackErr, errRollbackRefused, store.release())
		}
		trace := &tracer{store: store, request: request, start: time.Now(), artifact: artifact, manifest: manifest,
			predecessor: sha256.Sum256(validation.RawEntries[8]), elapsedOffset: validation.Entries[8].ElapsedNanos}
		if rollbackErr := store.rollbackToPredecessor(request.Generation, *selection.Rollback); rollbackErr != nil {
			return transactionInvalidResult(request.Generation), true, errors.Join(rollbackErr, store.release())
		}
		callErr := callBounded(context.Background(), trace.deadline(stateSelfTesting), func(callCtx context.Context) error {
			return request.SelfTest.Check(callCtx, rollbackIdentity(request, *selection.Rollback))
		})
		if callErr != nil {
			recordErr := trace.record(context.Background(), "12-repair-required", stateRepairRequired, adapterFailed)
			if recordErr != nil {
				return transactionInvalidResult(request.Generation), true, errors.Join(callErr, recordErr, store.release())
			}
			return repairRequiredResult(request.Generation, selection.Rollback.Artifact, inspection.currentCustody), true,
				errors.Join(callErr, errRepairRequired, store.release())
		}
		if recordErr := trace.record(context.Background(), "11-rolled-back", stateRolledBack, adapterSuccess); recordErr != nil {
			return transactionInvalidResult(request.Generation), true, errors.Join(recordErr, store.release())
		}
		return rolledBackResult(request.Generation, selection.Rollback.Artifact, inspection.currentCustody), true,
			errors.Join(errRolledBack, store.release())
	}
	return selfTestFailedResult(request.Generation, artifact, selection.Rollback.Artifact, inspection.currentCustody), true,
		errors.Join(errRollbackPending, store.release())
}

func validateRollbackDecision(store *ownedStore, request Request, selection currentSelection) error {
	decision := request.RollbackDecision
	if selection.Rollback == nil || (decision.Outcome != releasedecision.Outcome("release-accepted") &&
		decision.Outcome != releasedecision.Outcome("no-update")) || decision.BuildSafety != "release-accepted" ||
		decision.Protocol != "release-accepted" || decision.ReferenceTime.Before(request.Decision.ReferenceTime) ||
		!decision.BuildSafetyNoNewWorkAfter.After(decision.ReferenceTime) ||
		!decision.BuildSafetyTerminateAfter.After(decision.BuildSafetyNoNewWorkAfter) {
		return errRollbackRefused
	}
	payload, err := readExactFile(filepath.Join(store.generationPath("generations", selection.Rollback.Generation), "artifact"), int(selection.Rollback.Length))
	if err != nil || sha256.Sum256(payload) != selection.Rollback.Artifact {
		return errors.Join(errRollbackRefused, err)
	}
	retained := request
	retained.Generation = selection.Rollback.Generation
	retained.Decision = decision
	retained.Artifact = payload
	manifest, err := readBoundedFile(filepath.Join(store.generationPath("generations", selection.Rollback.Generation), "manifest.bin"), maximumRecordBytes)
	if err != nil || sha256.Sum256(manifest) != selection.Rollback.Manifest {
		return errors.Join(errRollbackRefused, err)
	}
	view, decodeErr := decodeManifest(manifest)
	if decodeErr != nil {
		return errors.Join(errRollbackRefused, decodeErr)
	}
	encoded, err := encodeManifestWithNotice(retained, sha256.Sum256(payload), view.SafeNotice)
	if err != nil {
		return errors.Join(errRollbackRefused, err)
	}
	if !bytes.Equal(encoded, manifest) {
		return errors.Join(errRollbackRefused, err)
	}
	return nil
}

func rollbackRefusedResult(generation uint64, current, rollback [32]byte, custody string) Result {
	return Result{Outcome: "rollback-refused", State: "repair-required", Generation: generation,
		CurrentDigest: current, RollbackDigest: rollback, StagingPresent: false,
		SafeNotice: "update rollback refused", CustodyNotice: custody}
}

var errRolledBack = errors.New("update rolled back")
var errRepairRequired = errors.New("update repair required")

func rolledBackResult(generation uint64, current [32]byte, custody string) Result {
	return Result{Outcome: "rolled-back", State: "rolled-back", Generation: generation, CurrentDigest: current,
		StagingPresent: false, SafeNotice: "update rolled back", CustodyNotice: custody}
}

func repairRequiredResult(generation uint64, current [32]byte, custody string) Result {
	return Result{Outcome: "repair-required", State: "repair-required", Generation: generation, CurrentDigest: current,
		StagingPresent: false, SafeNotice: "update repair required", CustodyNotice: custody}
}

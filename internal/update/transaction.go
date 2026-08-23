package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

const invalidOutcome = "release-invalid"

// applyInterruptionControl is the private per-invocation Apply
// checkpoint control. Public Apply calls applyWithInterruption with
// nil; only tests may inject one bounded control value. Each
// invocation is independent: the control is not package-global,
// context-carried, or exported, and a real crash sentinel preserves
// every checkpoint byte without running normal failure cleanup.
type applyInterruptionControl struct {
	StopBefore func(name string) bool
	StopAfter  func(name string) bool
}

var errApplyInterrupted = errors.New("update transaction interrupted at test checkpoint")
var errCandidateMismatch = errors.New("update candidate does not match accepted decision")
var errResourceDenied = errors.New("update resource envelope is unavailable")

type tracer struct {
	store         *ownedStore
	request       Request
	start         time.Time
	artifact      [32]byte
	manifest      [32]byte
	predecessor   [32]byte
	callerLimit   time.Time
	elapsedOffset uint64
	control       *applyInterruptionControl
}

// Apply executes one complete accepted offline update transaction. It
// acquires the permanent OS lock at entry and releases it at exit so
// concurrent Recover calls detect busy ownership. The Module never
// creates, repairs, replaces, retries, or unlinks the lock.
func Apply(ctx context.Context, request Request) (Result, error) {
	start := time.Now()
	callerLimit, _ := contextDeadline(ctx)
	return applyWithControls(ctx, request, nil, stageOperations{}, nil, start, callerLimit)
}

// applyWithInterruption runs the complete Apply orchestration with
// the supplied private per-invocation control. Public Apply calls this
// with nil; tests may inject a non-nil control to stop execution
// immediately before or after every accepted checkpoint. A non-nil
// control that signals a stop bypasses normal failure cleanup and
// normal Result construction, releases only the process-owned OS-lock
// handle, and preserves every checkpoint byte on disk.
func applyWithInterruption(ctx context.Context, request Request, control *applyInterruptionControl) (result Result, resultErr error) {
	start := time.Now()
	callerLimit, _ := contextDeadline(ctx)
	return applyWithControls(ctx, request, control, stageOperations{}, nil, start, callerLimit)
}

// applyWithStageOperations is the private per-invocation S7.2-03 staging
// operation seam. Public Apply always supplies the native zero value. Tests
// may replace only candidate write/flush/close, directory rename, and parent
// acknowledgement; root admission, records, cleanup, clocks, and Results stay
// production-owned.
func applyWithStageOperations(ctx context.Context, request Request, operations stageOperations) (Result, error) {
	start := time.Now()
	callerLimit, _ := contextDeadline(ctx)
	return applyWithControls(ctx, request, nil, operations, nil, start, callerLimit)
}

// applyWithResourceObservation is the private S7.2-03 resource seam. Tests
// may supply one already-observed native value; they cannot replace path
// admission, arithmetic, journal construction, cleanup, or Result mapping.
func applyWithResourceObservation(ctx context.Context, request Request, observation resourceObservation) (Result, error) {
	start := time.Now()
	callerLimit, _ := contextDeadline(ctx)
	return applyWithControls(ctx, request, nil, stageOperations{}, func(string) (resourceObservation, error) {
		return observation, nil
	}, start, callerLimit)
}

// applyWithStart is the private per-invocation monotonic-clock seam for the
// bounded-work oracle. It changes only the invocation start used to derive
// deadlines; all storage, cleanup, and public Apply dependencies remain real.
func applyWithStart(ctx context.Context, request Request, start time.Time) (Result, error) {
	callerLimit, _ := contextDeadline(ctx)
	return applyWithControls(ctx, request, nil, stageOperations{}, nil, start, callerLimit)
}

func applyWithControls(ctx context.Context, request Request, control *applyInterruptionControl, operations stageOperations,
	observe func(string) (resourceObservation, error), start, callerLimit time.Time) (result Result, resultErr error) {
	var authorized bool
	request, authorized = authorizedRequest(request)
	if !authorized {
		return invalidResult(request, "release-accepted"), errRecordInvalid
	}
	if request.schemaPlan == "" {
		request.schemaPlan = "no-op-v1"
		if request.Schema != nil {
			request.schemaPlan = "copy-on-write-v1"
		}
	}
	artifact, _, _, err := validateRequest(ctx, request)
	if err != nil {
		if errors.Is(err, errCandidateMismatch) {
			return stagingFailureResult(request, "release-accepted"), err
		}
		if errors.Is(err, errResourceDenied) {
			return resourceDeniedResult(request), err
		}
		return invalidResult(request, "release-accepted"), err
	}
	lock, lockErr := acquireOwnedLock(request.UpdateRoot)
	if lockErr != nil {
		if errors.Is(lockErr, errLockBusy) {
			return Result{Outcome: "resource-denied", State: "busy", Generation: 0, StagingPresent: false, SafeNotice: "update transaction busy"}, lockErr
		}
		return invalidResult(request, "release-accepted"), lockErr
	}
	defer func() {
		releaseErr := lock.release()
		if releaseErr == nil {
			return
		}
		if resultErr == nil {
			result = invalidResult(request, result.State)
		}
		resultErr = errors.Join(resultErr, releaseErr)
	}()
	if observe == nil {
		observe = observeOwnedStorage
	}
	observation, observeErr := observe(request.UpdateRoot)
	if observeErr != nil {
		if errors.Is(observeErr, errCapacityObservation) {
			return resourceDeniedResult(request), observeErr
		}
		return activationUnsupportedResult(request), observeErr
	}
	if occupied, present, occupiedErr := occupiedStagingResult(request, request.UpdateRoot); occupiedErr != nil {
		return transactionInvalidResult(request.generation), occupiedErr
	} else if present {
		return occupied, errResourceDenied
	}
	store, inspection, generation, err := openTransaction(request.UpdateRoot, request.generation, artifact)
	if err != nil {
		return transactionInvalidResult(generation), err
	}
	request.generation = generation
	manifestBytes, err := encodeManifest(request, artifact)
	if err != nil {
		return invalidResult(request, "release-accepted"), errors.Join(err, store.release())
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	if result, matched := committedRequest(store, inspection, request, artifact, manifestDigest); matched {
		if err := store.release(); err != nil {
			return invalidResult(request, "committed"), err
		}
		return result, nil
	}
	if result, handled, resumeErr := resumeUnavailableSelfTest(ctx, store, inspection, request, artifact, manifestDigest, start, callerLimit); handled {
		return result, resumeErr
	}
	if result, handled, resumeErr := resumeSuccessfulSelfTest(ctx, store, inspection, request, artifact, manifestDigest, start); handled {
		return result, resumeErr
	}
	if result, handled, resumeErr := resumeRollbackPending(store, inspection, request, artifact, manifestDigest); handled {
		return result, resumeErr
	}
	if inspection.selection.Transaction+1 != request.generation {
		return applyFailure(store, request, "release-accepted", false, errRecordInvalid)
	}
	schema, schemaErr := planSchema(ctx, request, inspection)
	if schemaErr != nil {
		if errors.Is(schemaErr, errResourceDenied) {
			return resourceDeniedResult(request), errors.Join(schemaErr, store.release())
		}
		return invalidResult(request, "release-accepted"), errors.Join(schemaErr, store.release())
	}
	successorCurrent, encodeErr := encodeCurrent(currentSelection{Transaction: request.generation,
		Current:  inspectedTuple{Generation: request.generation, Length: uint64(len(request.Artifact)), Artifact: artifact, Manifest: manifestDigest},
		Rollback: &inspection.selection.Current})
	if encodeErr != nil {
		return invalidResult(request, "release-accepted"), errors.Join(encodeErr, store.release())
	}
	resourceParts := schema.resourceParts()
	if inspection.selection.Rollback != nil {
		retirement, retirementErr := encodeRollbackRetirement(inspection.currentRaw)
		if retirementErr != nil {
			return invalidResult(request, "release-accepted"), errors.Join(retirementErr, store.release())
		}
		resourceParts = append(resourceParts, retirement)
	}
	if envelopeErr := requireResourceEnvelope(observation, request.Artifact, manifestBytes, successorCurrent, resourceParts...); envelopeErr != nil {
		return resourceDeniedResult(request), errors.Join(envelopeErr, store.release())
	}
	if inspection.selection.Rollback != nil {
		store, inspection, err = retireForNextGeneration(store, inspection, request.generation)
		if err != nil {
			return transactionInvalidResult(request.generation), err
		}
	}
	predecessorBytes, err := encodePredecessor(inspection.predecessor)
	if err != nil {
		return applyFailure(store, request, "release-accepted", false, err)
	}
	trace := &tracer{store: store, request: request, start: start, artifact: artifact, manifest: manifestDigest,
		predecessor: sha256.Sum256(predecessorBytes), control: control, callerLimit: callerLimit}
	if operations.openFile == nil || operations.renameDirectory == nil || operations.acknowledge == nil {
		operations = nativeStageOperations(store.ops)
	}
	if err := store.prepare(request.generation); err != nil {
		return applyFailure(store, request, "release-accepted", true, err)
	}
	if err := trace.record(ctx, "01-release-accepted", stateReleaseAccepted, adapterNotCalled); err != nil {
		return applyFailure(store, request, "release-accepted", true, err)
	}
	if err := trace.record(ctx, "02-artifact-verified", stateArtifactVerified, adapterNotCalled); err != nil {
		return applyFailure(store, request, "artifact-verified", true, err)
	}
	if err := store.stage(request.generation, request.Artifact, manifestBytes, operations); err != nil {
		return stageApplyFailure(store, request, inspection, "artifact-verified", err)
	}
	if err := trace.record(ctx, "03-staged", stateStaged, adapterNotCalled); err != nil {
		return applyFailure(store, request, "staged", true, err)
	}
	if err := trace.record(ctx, "04-rollback-reserved", stateRollbackReserved, adapterNotCalled); err != nil {
		return applyFailure(store, request, "rollback-reserved", true, err)
	}
	if err := callBounded(ctx, trace.deadline(stateStopNewWork), func(callCtx context.Context) error {
		return stopRuntimeWork(callCtx, request.Work)
	}); err != nil {
		recordErr := trace.record(context.Background(), "05-stop-new-work", stateStopNewWork, adapterFailed)
		if recordErr != nil {
			return applyFailure(store, request, "rollback-reserved", true, errors.Join(err, recordErr))
		}
		return drainFailure(store, request, inspection, "rollback-reserved", err)
	}
	if err := trace.record(ctx, "05-stop-new-work", stateStopNewWork, adapterSuccess); err != nil {
		return applyFailure(store, request, "stop-new-work", true, err)
	}
	if err := callBounded(ctx, trace.deadline(stateDraining), func(callCtx context.Context) error {
		return drainRuntimeWork(callCtx, request.Work)
	}); err != nil {
		recordErr := trace.record(context.Background(), "06-draining", stateDraining, adapterFailed)
		if recordErr != nil {
			return applyFailure(store, request, "stop-new-work", true, errors.Join(err, recordErr))
		}
		return drainFailure(store, request, inspection, "stop-new-work", err)
	}
	if err := trace.record(ctx, "06-draining", stateDraining, adapterSuccess); err != nil {
		return applyFailure(store, request, "draining", true, err)
	}
	if err := schema.prepare(ctx); err != nil {
		return schemaPreparationFailure(store, request, inspection, &schema, err)
	}
	if _, revalidateErr := observeOwnedStorage(request.UpdateRoot); revalidateErr != nil {
		recordErr := trace.record(context.Background(), "07-activated", stateActivated, adapterUnavailable)
		if recordErr != nil {
			return applyFailure(store, request, "draining", true, errors.Join(revalidateErr, recordErr))
		}
		return activationRefusal(store, request, inspection, revalidateErr)
	}
	current := inspectedTuple{Generation: request.generation, Length: uint64(len(request.Artifact)), Artifact: artifact, Manifest: manifestDigest}
	selection := currentSelection{Transaction: request.generation, Current: current, Rollback: &inspection.selection.Current}
	if err := store.activate(request.generation, selection, inspection.predecessor.CurrentRecordDigest, control); err != nil {
		if isActivationBusy(err) {
			return activationBusyFailure(store, request, inspection, err)
		}
		return applyFailure(store, request, "draining", false, err)
	}
	if err := trace.record(ctx, "07-activated", stateActivated, adapterNotCalled); err != nil {
		return applyFailure(store, request, "activated", false, err)
	}
	identity := CandidateIdentity{Generation: request.generation, TargetPath: request.decision.Path, Length: request.decision.Length,
		Digest: artifact, Platform: request.decision.Platform, Architecture: request.decision.Architecture, Environment: request.decision.Environment, Network: request.decision.Network}
	if err := callBounded(ctx, trace.deadline(stateSelfTesting), func(callCtx context.Context) error {
		return request.SelfTest.Check(callCtx, identity)
	}); err != nil {
		if selfTestUnavailableOnly(err) {
			recordErr := trace.record(context.Background(), "08-self-testing", stateSelfTesting, adapterUnavailable)
			if recordErr == nil {
				result := networkingUnverifiedResult(request.generation, artifact, inspection.selection.Current.Artifact)
				return result, errors.Join(err, store.release())
			}
			return applyFailure(store, request, "self-testing", false, errors.Join(err, recordErr))
		}
		recordErr := trace.record(ctx, "08-self-testing", stateSelfTesting, adapterFailed)
		if recordErr != nil {
			return applyFailure(store, request, "self-testing", false, errors.Join(err, recordErr))
		}
		pendingErr := trace.record(context.Background(), "10-rollback-pending", stateRollbackPending, adapterNotCalled)
		if pendingErr != nil {
			return applyFailure(store, request, "self-testing", false, errors.Join(err, pendingErr))
		}
		result := selfTestFailedResult(request.generation, artifact, inspection.selection.Current.Artifact)
		return result, errors.Join(err, store.release())
	}
	if err := callBounded(ctx, trace.deadline(stateSelfTesting), func(callCtx context.Context) error {
		return rejoinRuntimeWork(callCtx, request.Work)
	}); err != nil {
		return rejoinFailure(ctx, trace, store, request, inspection, artifact, err)
	}
	if err := trace.record(ctx, "08-self-testing", stateSelfTesting, adapterSuccess); err != nil {
		return applyFailure(store, request, "self-testing", false, err)
	}
	if err := schema.commit(store); err != nil {
		return applyFailure(store, request, "self-testing", false, err)
	}
	if err := trace.record(ctx, "09-committed", stateCommitted, adapterNotCalled); err != nil {
		return applyFailure(store, request, "committed", false, err)
	}
	result = committedResult(request.generation, artifact, inspection.selection.Current.Artifact, "update committed")
	if err := store.release(); err != nil {
		return invalidResult(request, "committed"), err
	}
	return result, nil
}

func validateRequest(ctx context.Context, request Request) ([32]byte, []byte, [32]byte, error) {
	var artifact, manifestDigest [32]byte
	if ctx == nil || ctx.Err() != nil ||
		request.decision.Outcome != release.OutcomeReleaseAccepted {
		return artifact, nil, manifestDigest, errRecordInvalid
	}
	if request.decision.Length > maximumArtifactBytes || int64(len(request.Artifact)) > maximumArtifactBytes {
		return artifact, nil, manifestDigest, errResourceDenied
	}
	if (request.schemaPlan != "no-op-v1" && request.schemaPlan != "copy-on-write-v1") || request.Work == nil || request.SelfTest == nil ||
		request.decision.BuildSafety != release.OutcomeReleaseAccepted ||
		request.decision.Protocol != release.OutcomeReleaseAccepted || request.decision.Length <= 0 ||
		len(request.decision.Digest) != sha256.Size || !completeFloors(request.decision.Floors) {
		return artifact, nil, manifestDigest, errRecordInvalid
	}
	if int64(len(request.Artifact)) != request.decision.Length {
		return artifact, nil, manifestDigest, errCandidateMismatch
	}
	artifact = sha256.Sum256(request.Artifact)
	var expected [32]byte
	copy(expected[:], request.decision.Digest)
	if artifact != expected {
		return artifact, nil, manifestDigest, errCandidateMismatch
	}
	if request.decision.Path == "" || len(request.decision.Path) > maximumTargetBytes ||
		request.decision.ReleaseVersion <= 0 || request.decision.ReferenceTime.IsZero() ||
		!request.decision.BuildSafetyNoNewWorkAfter.After(request.decision.ReferenceTime) ||
		!request.decision.BuildSafetyTerminateAfter.After(request.decision.BuildSafetyNoNewWorkAfter) ||
		request.decision.RootVersion != request.decision.Floors.RootVersion {
		return artifact, nil, manifestDigest, errRecordInvalid
	}
	manifest, err := encodeManifest(request, artifact)
	if err != nil {
		return artifact, nil, manifestDigest, err
	}
	return artifact, manifest, sha256.Sum256(manifest), nil
}

func stagingFailureResult(request Request, state string) Result {
	return Result{Outcome: "staging-failed", State: state, Generation: request.generation,
		StagingPresent: false, SafeNotice: "update staging failed"}
}

func stageApplyFailure(store *ownedStore, request Request, inspection rootInspection, state string, cause error) (Result, error) {
	cleanupErr := store.cleanup(request.generation)
	releaseErr := store.release()
	if cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: state, Generation: request.generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, cleanupErr, releaseErr)
	}
	result := stagingFailureResult(request, state)
	result.CurrentDigest = inspection.selection.Current.Artifact
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	return result, errors.Join(cause, releaseErr)
}

// occupiedStagingResult classifies an already-present staging candidate using
// the same bounded journal evidence as restart recovery. Apply never repairs
// or cleans this evidence: a coherent candidate owns the slot, while any
// ambiguity remains transaction-invalid.
func occupiedStagingResult(request Request, root string) (Result, bool, error) {
	entries, readErr := os.ReadDir(filepath.Join(root, "staging"))
	if readErr != nil {
		return Result{}, false, readErr
	}
	if len(entries) == 0 {
		return Result{}, false, nil
	}
	facts, err := collectInventory(root)
	if err != nil {
		return Result{}, false, err
	}
	if len(facts.StagingDirs) != 1 || len(facts.Transactions) != 1 {
		return Result{}, false, errRecordInvalid
	}
	generation := facts.Transactions[0].Generation
	if facts.StagingDirs[0].Generation != generation {
		return Result{}, false, errRecordInvalid
	}
	records, err := reconstructRecoveryRecords(facts)
	if err != nil {
		return Result{}, false, err
	}
	artifact, manifest := candidateCommitments(facts, generation)
	validation, err := validateJournal(generation, facts.journalLookup(generation), records.predecessorCommitment, artifact, manifest)
	if err != nil {
		return Result{}, false, err
	}
	plan, err := planRecovery(facts, validation, records)
	if err != nil {
		return Result{}, false, errors.Join(errRecordInvalid, err)
	}
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return Result{}, false, err
	}
	result := Result{Outcome: "resource-denied", State: plan.State, Generation: generation,
		CurrentDigest: selection.Current.Artifact, StagingPresent: true,
		SafeNotice: "update recovery required"}
	if selection.Rollback != nil {
		result.RollbackDigest = selection.Rollback.Artifact
	}
	return result, true, nil
}

// record writes one journal entry, observes the bound, and notifies the
// private Apply checkpoint control before and after each publication.
// A non-nil control that signals a stop via StopBefore returns a sentinel
// cancellation error so the caller preserves the bytes on disk and
// releases only the lock.
func committedRequest(store *ownedStore, inspection rootInspection, request Request,
	artifact, manifest [32]byte) (Result, bool) {
	selection := inspection.selection
	if selection.Transaction != request.generation || selection.Rollback == nil ||
		selection.Current.Artifact != artifact || selection.Current.Manifest != manifest {
		return Result{}, false
	}
	directory := filepath.Join(store.generationPath("transactions", request.generation), "journal")
	first, err := readExactFile(filepath.Join(directory, "01-release-accepted.entry"), journalRecordBytes)
	if err != nil {
		return Result{}, false
	}
	entry, err := decodeJournalEntry(first)
	if err != nil {
		return Result{}, false
	}
	if _, err := inspectJournal(directory, request.generation, artifact, manifest, entry.Predecessor); err != nil {
		return Result{}, false
	}
	return committedResult(request.generation, artifact, selection.Rollback.Artifact, "update committed"), true
}

func committedResult(generation uint64, current, rollback [32]byte, safe string) Result {
	return Result{Outcome: "committed", State: "committed", Generation: generation,
		CurrentDigest: current, RollbackDigest: rollback, SafeNotice: safe}
}

func invalidResult(request Request, state string) Result {
	return Result{Outcome: invalidOutcome, State: state, Generation: request.generation,
		StagingPresent: false, SafeNotice: "update transaction rejected"}
}

func transactionInvalidResult(generation uint64) Result {
	return Result{Outcome: "transaction-invalid", State: "transaction-invalid", Generation: generation,
		StagingPresent: false, SafeNotice: "update transaction invalid"}
}

func failApply(store *ownedStore, request Request, state string, cleanup bool, cause error) (Result, error) {
	var cleanupErr error
	if cleanup {
		cleanupErr = store.cleanup(request.generation)
	}
	releaseErr := store.release()
	if cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: state, Generation: request.generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, cleanupErr, releaseErr)
	}
	return invalidResult(request, state), errors.Join(cause, releaseErr)
}

func applyFailure(store *ownedStore, request Request, state string, cleanup bool, cause error) (Result, error) {
	if errors.Is(cause, errApplyInterrupted) {
		return Result{}, cause
	}
	return failApply(store, request, state, cleanup, cause)
}

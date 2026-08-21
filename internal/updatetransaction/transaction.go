package updatetransaction

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"time"
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

type tracer struct {
	store       *ownedStore
	request     Request
	start       time.Time
	artifact    [32]byte
	manifest    [32]byte
	predecessor [32]byte
	callerLimit time.Time
	control     *applyInterruptionControl
}

// Apply executes one complete accepted offline update transaction. It
// acquires the permanent OS lock at entry and releases it at exit so
// concurrent Recover calls detect busy ownership. The Module never
// creates, repairs, replaces, retries, or unlinks the lock.
func Apply(ctx context.Context, request Request) (Result, error) {
	return applyWithInterruption(ctx, request, nil)
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
	artifact, manifestBytes, manifestDigest, err := validateRequest(ctx, request)
	if err != nil {
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
	store, inspection, err := acquireStore(request.UpdateRoot, request.Generation)
	if err != nil {
		return invalidResult(request, "release-accepted"), err
	}
	if result, matched := committedRequest(store, inspection, request, artifact, manifestDigest); matched {
		if err := store.release(); err != nil {
			return invalidResult(request, "committed"), err
		}
		return result, nil
	}
	if inspection.selection.Transaction+1 != request.Generation || inspection.selection.Rollback != nil {
		return applyFailure(store, request, "release-accepted", false, errRecordInvalid)
	}
	predecessorBytes, err := encodePredecessor(inspection.predecessor)
	if err != nil {
		return applyFailure(store, request, "release-accepted", false, err)
	}
	trace := &tracer{store: store, request: request, start: start, artifact: artifact, manifest: manifestDigest,
		predecessor: sha256.Sum256(predecessorBytes), control: control}
	trace.callerLimit, _ = ctx.Deadline()
	if err := store.prepare(request.Generation); err != nil {
		return applyFailure(store, request, "release-accepted", true, err)
	}
	if err := trace.record(ctx, "01-release-accepted", stateReleaseAccepted, adapterNotCalled); err != nil {
		return applyFailure(store, request, "release-accepted", true, err)
	}
	if err := trace.record(ctx, "02-artifact-verified", stateArtifactVerified, adapterNotCalled); err != nil {
		return applyFailure(store, request, "artifact-verified", true, err)
	}
	if err := store.stage(request.Generation, request.Artifact, manifestBytes); err != nil {
		return applyFailure(store, request, "artifact-verified", true, err)
	}
	if err := trace.record(ctx, "03-staged", stateStaged, adapterNotCalled); err != nil {
		return applyFailure(store, request, "staged", true, err)
	}
	if err := trace.record(ctx, "04-rollback-reserved", stateRollbackReserved, adapterNotCalled); err != nil {
		return applyFailure(store, request, "rollback-reserved", true, err)
	}
	if err := callBounded(ctx, trace.deadline(stateStopNewWork), request.Work.StopNewWork); err != nil {
		recordErr := trace.record(ctx, "05-stop-new-work", stateStopNewWork, adapterFailed)
		return applyFailure(store, request, "stop-new-work", true, errors.Join(err, recordErr))
	}
	if err := trace.record(ctx, "05-stop-new-work", stateStopNewWork, adapterSuccess); err != nil {
		return applyFailure(store, request, "stop-new-work", true, err)
	}
	if err := callBounded(ctx, trace.deadline(stateDraining), request.Work.Drain); err != nil {
		recordErr := trace.record(ctx, "06-draining", stateDraining, adapterFailed)
		return applyFailure(store, request, "draining", true, errors.Join(err, recordErr))
	}
	if err := trace.record(ctx, "06-draining", stateDraining, adapterSuccess); err != nil {
		return applyFailure(store, request, "draining", true, err)
	}
	current := inspectedTuple{Generation: request.Generation, Length: uint64(len(request.Artifact)), Artifact: artifact, Manifest: manifestDigest}
	selection := currentSelection{Transaction: request.Generation, Current: current, Rollback: &inspection.selection.Current}
	if err := store.activate(request.Generation, selection, inspection.predecessor.CurrentRecordDigest, control); err != nil {
		return applyFailure(store, request, "draining", false, err)
	}
	if err := trace.record(ctx, "07-activated", stateActivated, adapterNotCalled); err != nil {
		return applyFailure(store, request, "activated", false, err)
	}
	identity := CandidateIdentity{Generation: request.Generation, TargetPath: request.Decision.Path, Length: request.Decision.Length,
		Digest: artifact, Platform: request.Decision.Platform, Architecture: request.Decision.Architecture, Environment: request.Decision.Environment, Network: request.Decision.Network}
	if err := callBounded(ctx, trace.deadline(stateSelfTesting), func(callCtx context.Context) error {
		return request.SelfTest.Check(callCtx, identity)
	}); err != nil {
		recordErr := trace.record(ctx, "08-self-testing", stateSelfTesting, adapterFailed)
		return applyFailure(store, request, "self-testing", false, errors.Join(err, recordErr))
	}
	if err := trace.record(ctx, "08-self-testing", stateSelfTesting, adapterSuccess); err != nil {
		return applyFailure(store, request, "self-testing", false, err)
	}
	if err := trace.record(ctx, "09-committed", stateCommitted, adapterNotCalled); err != nil {
		return applyFailure(store, request, "committed", false, err)
	}
	result = committedResult(request.Generation, artifact, inspection.selection.Current.Artifact, "update committed", request.Decision.CustodyNotice)
	if err := store.release(); err != nil {
		return invalidResult(request, "committed"), err
	}
	return result, nil
}

func validateRequest(ctx context.Context, request Request) ([32]byte, []byte, [32]byte, error) {
	var artifact, manifestDigest [32]byte
	if ctx == nil || ctx.Err() != nil || request.Generation == 0 || request.ActiveWork != 0 ||
		request.SchemaPlan != "no-op-v1" || request.Work == nil || request.SelfTest == nil ||
		request.Decision.Outcome != "release-accepted" || request.Decision.BuildSafety != "release-accepted" ||
		request.Decision.Protocol != "release-accepted" || request.Decision.Length <= 0 ||
		request.Decision.Length > maximumArtifactBytes || int64(len(request.Artifact)) != request.Decision.Length ||
		len(request.Decision.Digest) != sha256.Size || !completeFloors(request.Decision.Floors) {
		return artifact, nil, manifestDigest, errRecordInvalid
	}
	artifact = sha256.Sum256(request.Artifact)
	var expected [32]byte
	copy(expected[:], request.Decision.Digest)
	if artifact != expected || request.Decision.Path == "" || len(request.Decision.Path) > maximumTargetBytes ||
		request.Decision.ReleaseVersion <= 0 || request.Decision.ReferenceTime.IsZero() ||
		!request.Decision.BuildSafetyNoNewWorkAfter.After(request.Decision.ReferenceTime) ||
		!request.Decision.BuildSafetyTerminateAfter.After(request.Decision.BuildSafetyNoNewWorkAfter) ||
		request.Decision.RootVersion != request.Decision.Floors.RootVersion {
		return artifact, nil, manifestDigest, errRecordInvalid
	}
	manifest, err := encodeManifest(request, artifact)
	if err != nil {
		return artifact, nil, manifestDigest, err
	}
	return artifact, manifest, sha256.Sum256(manifest), nil
}

// record writes one journal entry, observes the bound, and notifies the
// private Apply checkpoint control before and after each publication.
// A non-nil control that signals a stop via StopBefore returns a sentinel
// cancellation error so the caller preserves the bytes on disk and
// releases only the lock.
func (trace *tracer) record(ctx context.Context, name string, state transactionState, adapter adapterResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := applyCheckpoint(trace.control, true, name); err != nil {
		return err
	}
	entry := journalEntry{State: state, Generation: trace.request.Generation,
		Predecessor: trace.predecessor, ArtifactDigest: trace.artifact,
		ManifestCommitment: trace.manifest, AdapterResult: adapter,
		Observation: byte(state), ElapsedNanos: uint64(time.Since(trace.start)),
		DeadlineUnix: trace.deadline(state).Unix()}
	raw, err := trace.store.writeEntry(entry)
	if err == nil {
		trace.predecessor = sha256.Sum256(raw)
		if checkpointErr := applyCheckpoint(trace.control, false, name); checkpointErr != nil {
			return checkpointErr
		}
	}
	return err
}

func applyCheckpoint(control *applyInterruptionControl, before bool, name string) error {
	if control == nil {
		return nil
	}
	stop := control.StopAfter
	if before {
		stop = control.StopBefore
	}
	if stop != nil && stop(name) {
		return errApplyInterrupted
	}
	return nil
}

func (trace *tracer) deadline(state transactionState) time.Time {
	deadline := trace.request.Decision.BuildSafetyTerminateAfter
	if protocol := trace.request.Decision.ProtocolTransitionDeadline; !protocol.IsZero() && protocol.Before(deadline) {
		deadline = protocol
	}
	if state == stateStopNewWork && trace.request.Decision.BuildSafetyNoNewWorkAfter.Before(deadline) {
		deadline = trace.request.Decision.BuildSafetyNoNewWorkAfter
	}
	if !trace.callerLimit.IsZero() && trace.callerLimit.Before(deadline) {
		deadline = trace.callerLimit
	}
	return deadline
}

func callBounded(parent context.Context, deadline time.Time, call func(context.Context) error) error {
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	return call(ctx)
}

func committedRequest(store *ownedStore, inspection rootInspection, request Request,
	artifact, manifest [32]byte) (Result, bool) {
	selection := inspection.selection
	if selection.Transaction != request.Generation || selection.Rollback == nil ||
		selection.Current.Artifact != artifact || selection.Current.Manifest != manifest {
		return Result{}, false
	}
	directory := filepath.Join(store.generationPath("transactions", request.Generation), "journal")
	first, err := readExactFile(filepath.Join(directory, "01-release-accepted.entry"), journalRecordBytes)
	if err != nil {
		return Result{}, false
	}
	entry, err := decodeJournalEntry(first)
	if err != nil {
		return Result{}, false
	}
	if _, err := inspectJournal(directory, request.Generation, artifact, manifest, entry.Predecessor); err != nil {
		return Result{}, false
	}
	return committedResult(request.Generation, artifact, selection.Rollback.Artifact,
		"update committed", request.Decision.CustodyNotice), true
}

func committedResult(generation uint64, current, rollback [32]byte, safe, custody string) Result {
	return Result{Outcome: "committed", State: "committed", Generation: generation,
		CurrentDigest: current, RollbackDigest: rollback, SafeNotice: safe, CustodyNotice: custody}
}

func invalidResult(request Request, state string) Result {
	return Result{Outcome: invalidOutcome, State: state, Generation: request.Generation,
		StagingPresent: false, SafeNotice: "update transaction rejected",
		CustodyNotice: request.Decision.CustodyNotice}
}

func failApply(store *ownedStore, request Request, state string, cleanup bool, cause error) (Result, error) {
	if cleanup {
		cause = errors.Join(cause, store.cleanup(request.Generation))
	}
	return invalidResult(request, state), errors.Join(cause, store.release())
}

func applyFailure(store *ownedStore, request Request, state string, cleanup bool, cause error) (Result, error) {
	if errors.Is(cause, errApplyInterrupted) {
		return Result{}, cause
	}
	return failApply(store, request, state, cleanup, cause)
}

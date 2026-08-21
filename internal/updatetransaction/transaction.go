package updatetransaction

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"time"
)

const invalidOutcome = "release-invalid"

type tracer struct {
	store       *ownedStore
	request     Request
	start       time.Time
	artifact    [32]byte
	manifest    [32]byte
	predecessor [32]byte
	callerLimit time.Time
}

// Apply executes one complete accepted offline update transaction.
func Apply(ctx context.Context, request Request) (Result, error) {
	start := time.Now()
	artifact, manifestBytes, manifestDigest, err := validateRequest(ctx, request)
	if err != nil {
		return invalidResult(request, "release-accepted"), err
	}
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
		return failApply(store, request, "release-accepted", false, errRecordInvalid)
	}
	predecessorBytes, err := encodePredecessor(inspection.predecessor)
	if err != nil {
		return failApply(store, request, "release-accepted", false, err)
	}
	trace := &tracer{store: store, request: request, start: start, artifact: artifact, manifest: manifestDigest, predecessor: sha256.Sum256(predecessorBytes)}
	trace.callerLimit, _ = ctx.Deadline()
	if err := store.prepare(request.Generation); err != nil {
		return failApply(store, request, "release-accepted", true, err)
	}
	if err := trace.record(ctx, stateReleaseAccepted, adapterNotCalled); err != nil {
		return failApply(store, request, "release-accepted", true, err)
	}
	if err := trace.record(ctx, stateArtifactVerified, adapterNotCalled); err != nil {
		return failApply(store, request, "artifact-verified", true, err)
	}
	if err := store.stage(request.Generation, request.Artifact, manifestBytes); err != nil {
		return failApply(store, request, "artifact-verified", true, err)
	}
	if err := trace.record(ctx, stateStaged, adapterNotCalled); err != nil {
		return failApply(store, request, "staged", true, err)
	}
	if err := trace.record(ctx, stateRollbackReserved, adapterNotCalled); err != nil {
		return failApply(store, request, "rollback-reserved", true, err)
	}
	if err := callBounded(ctx, trace.deadline(stateStopNewWork), request.Work.StopNewWork); err != nil {
		recordErr := trace.record(ctx, stateStopNewWork, adapterFailed)
		return failApply(store, request, "stop-new-work", true, errors.Join(err, recordErr))
	}
	if err := trace.record(ctx, stateStopNewWork, adapterSuccess); err != nil {
		return failApply(store, request, "stop-new-work", true, err)
	}
	if err := callBounded(ctx, trace.deadline(stateDraining), request.Work.Drain); err != nil {
		recordErr := trace.record(ctx, stateDraining, adapterFailed)
		return failApply(store, request, "draining", true, errors.Join(err, recordErr))
	}
	if err := trace.record(ctx, stateDraining, adapterSuccess); err != nil {
		return failApply(store, request, "draining", true, err)
	}
	current := inspectedTuple{Generation: request.Generation, Length: uint64(len(request.Artifact)), Artifact: artifact, Manifest: manifestDigest}
	selection := currentSelection{Transaction: request.Generation, Current: current, Rollback: &inspection.selection.Current}
	if err := store.activate(request.Generation, selection, inspection.predecessor.CurrentRecordDigest); err != nil {
		return failApply(store, request, "draining", false, err)
	}
	if err := trace.record(ctx, stateActivated, adapterNotCalled); err != nil {
		return failApply(store, request, "activated", false, err)
	}
	identity := CandidateIdentity{Generation: request.Generation, TargetPath: request.Decision.Path, Length: request.Decision.Length,
		Digest: artifact, Platform: request.Decision.Platform, Architecture: request.Decision.Architecture, Environment: request.Decision.Environment, Network: request.Decision.Network}
	if err := callBounded(ctx, trace.deadline(stateSelfTesting), func(callCtx context.Context) error {
		return request.SelfTest.Check(callCtx, identity)
	}); err != nil {
		recordErr := trace.record(ctx, stateSelfTesting, adapterFailed)
		return failApply(store, request, "self-testing", false, errors.Join(err, recordErr))
	}
	if err := trace.record(ctx, stateSelfTesting, adapterSuccess); err != nil {
		return failApply(store, request, "self-testing", false, err)
	}
	if err := trace.record(ctx, stateCommitted, adapterNotCalled); err != nil {
		return failApply(store, request, "committed", false, err)
	}
	result := committedResult(request.Generation, artifact, inspection.selection.Current.Artifact, "update committed", request.Decision.CustodyNotice)
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

func (trace *tracer) record(ctx context.Context, state transactionState, adapter adapterResult) error {
	if err := ctx.Err(); err != nil {
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
	}
	return err
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

// Recover returns only a coherent terminal transaction; S7.2-02 adds interruption recovery.
func Recover(ctx context.Context, root string) (Result, error) {
	if ctx == nil || ctx.Err() != nil {
		return Result{Outcome: invalidOutcome}, errRecordInvalid
	}
	raw, err := readBoundedFile(filepath.Join(root, "current"), maximumRecordBytes)
	if err != nil {
		return Result{Outcome: invalidOutcome}, err
	}
	selected, err := decodeCurrent(raw)
	if err != nil || selected.Transaction == 0 {
		return Result{Outcome: invalidOutcome}, errors.Join(errRecordInvalid, err)
	}
	store, inspection, err := acquireStore(root, selected.Transaction)
	if err != nil {
		return Result{Outcome: invalidOutcome}, err
	}
	selection := inspection.selection
	if selection.Transaction != selected.Transaction || selection.Rollback == nil || ctx.Err() != nil {
		return Result{Outcome: invalidOutcome}, errors.Join(errRecordInvalid, store.release())
	}
	view, _, _, err := store.inspectPayload("generations", selection.Current)
	directory := filepath.Join(store.generationPath("transactions", selection.Transaction), "journal")
	first, firstErr := readExactFile(filepath.Join(directory, "01-release-accepted.entry"), journalRecordBytes)
	entry, entryErr := decodeJournalEntry(first)
	_, journalErr := inspectJournal(directory, selection.Transaction, selection.Current.Artifact,
		selection.Current.Manifest, entry.Predecessor)
	if err = errors.Join(err, firstErr, entryErr, journalErr); err != nil {
		return Result{Outcome: invalidOutcome}, errors.Join(err, store.release())
	}
	result := committedResult(selection.Transaction, selection.Current.Artifact,
		selection.Rollback.Artifact, view.SafeNotice, view.CustodyNotice)
	if err := store.release(); err != nil {
		return Result{Outcome: invalidOutcome}, err
	}
	return result, nil
}

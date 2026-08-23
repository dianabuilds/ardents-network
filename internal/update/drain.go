package update

import (
	"context"
	"errors"
	"time"
)

func stopRuntimeWork(ctx context.Context, work WorkControl) error {
	if err := work.StopNewWork(ctx); err != nil {
		return err
	}
	return work.StopNewAssignments(ctx)
}

func drainRuntimeWork(ctx context.Context, work WorkControl) error {
	if err := work.Drain(ctx); err != nil {
		return err
	}
	return work.DrainAssignments(ctx)
}

func rejoinRuntimeWork(ctx context.Context, work WorkControl) error {
	return work.RejoinOrWithdraw(ctx)
}

// deadline derives the one operation deadline from the invocation timestamp.
// It deliberately keeps the v1 journal deadline separate so bounded runtime
// behavior does not rewrite frozen journal bytes.
func (trace *tracer) deadline(state transactionState) time.Time {
	deadline := trace.start.Add(15 * time.Second)
	if documented := trace.journalDeadline(state); documented.Before(deadline) {
		deadline = documented
	}
	return deadline
}

func (trace *tracer) journalDeadline(state transactionState) time.Time {
	deadline := trace.request.decision.BuildSafetyTerminateAfter
	if protocol := trace.request.decision.ProtocolTransitionDeadline; !protocol.IsZero() && protocol.Before(deadline) {
		deadline = protocol
	}
	if state == stateStopNewWork && trace.request.decision.BuildSafetyNoNewWorkAfter.Before(deadline) {
		deadline = trace.request.decision.BuildSafetyNoNewWorkAfter
	}
	if !trace.callerLimit.IsZero() && trace.callerLimit.Before(deadline) {
		deadline = trace.callerLimit
	}
	return deadline
}

func contextDeadline(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	return ctx.Deadline()
}

func callBounded(parent context.Context, deadline time.Time, call func(context.Context) error) error {
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	err := call(ctx)
	if contextErr := ctx.Err(); contextErr != nil {
		return errors.Join(err, contextErr)
	}
	return err
}

// drainFailure preserves only independently inspected predecessor facts after
// a bounded WorkControl refusal. The candidate and its transaction evidence
// are removed before returning; incomplete cleanup exposes no stale facts.
func drainFailure(store *ownedStore, request Request, inspection rootInspection, state string, cause error) (Result, error) {
	cleanupErr := store.cleanup(request.generation)
	releaseErr := store.release()
	if cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: state, Generation: request.generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, cleanupErr, releaseErr)
	}
	result := Result{Outcome: "drain-expired", State: state, Generation: request.generation,
		CurrentDigest: inspection.selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update drain expired", EvidenceNotice: inspection.currentEvidence}
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	return result, errors.Join(cause, releaseErr)
}

func activationRefusal(store *ownedStore, request Request, inspection rootInspection, cause error) (Result, error) {
	cleanupErr := store.cleanup(request.generation)
	releaseErr := store.release()
	if cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: "draining", Generation: request.generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, cleanupErr, releaseErr)
	}
	result := Result{Outcome: "activation-unsupported", State: "draining", Generation: request.generation,
		CurrentDigest: inspection.selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update storage unsupported", EvidenceNotice: inspection.currentEvidence}
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	return result, errors.Join(cause, releaseErr)
}

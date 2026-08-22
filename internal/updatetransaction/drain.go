package updatetransaction

import (
	"context"
	"errors"
	"time"
)

// contributorWorkControl is an optional capability of the existing runtime
// Adapter. Keeping it private preserves the small public WorkControl seam:
// runtimes without Contributor duties keep their exact S7.2-01 behavior.
// A Contributor-capable Adapter owns role identities, lease deadlines, and
// terminal inventories; Update Transaction supplies only its already bounded
// operation context.
type contributorWorkControl interface {
	StopNewAssignments(context.Context) error
	DrainAssignments(context.Context) error
	RejoinOrWithdraw(context.Context) error
}

func stopRuntimeWork(ctx context.Context, work WorkControl) error {
	if err := work.StopNewWork(ctx); err != nil {
		return err
	}
	if contributor, ok := work.(contributorWorkControl); ok {
		return contributor.StopNewAssignments(ctx)
	}
	return nil
}

func drainRuntimeWork(ctx context.Context, work WorkControl) error {
	if err := work.Drain(ctx); err != nil {
		return err
	}
	if contributor, ok := work.(contributorWorkControl); ok {
		return contributor.DrainAssignments(ctx)
	}
	return nil
}

func rejoinRuntimeWork(ctx context.Context, work WorkControl) error {
	if contributor, ok := work.(contributorWorkControl); ok {
		return contributor.RejoinOrWithdraw(ctx)
	}
	return nil
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
	cleanupErr := store.cleanup(request.Generation)
	releaseErr := store.release()
	if cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: state, Generation: request.Generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, cleanupErr, releaseErr)
	}
	result := Result{Outcome: "drain-expired", State: state, Generation: request.Generation,
		CurrentDigest: inspection.selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update drain expired", CustodyNotice: inspection.currentCustody}
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	return result, errors.Join(cause, releaseErr)
}

func activationRefusal(store *ownedStore, request Request, inspection rootInspection, cause error) (Result, error) {
	cleanupErr := store.cleanup(request.Generation)
	releaseErr := store.release()
	if cleanupErr != nil {
		return Result{Outcome: "cleanup-incomplete", State: "draining", Generation: request.Generation,
			StagingPresent: false, SafeNotice: "update cleanup incomplete"}, errors.Join(cause, cleanupErr, releaseErr)
	}
	result := Result{Outcome: "activation-unsupported", State: "draining", Generation: request.Generation,
		CurrentDigest: inspection.selection.Current.Artifact, StagingPresent: false,
		SafeNotice: "update storage unsupported", CustodyNotice: inspection.currentCustody}
	if inspection.selection.Rollback != nil {
		result.RollbackDigest = inspection.selection.Rollback.Artifact
	}
	return result, errors.Join(cause, releaseErr)
}

//go:build linux

package endpoint

import (
	"context"
	"errors"
)

// sourcePreparationFailure marks the local preparation boundary that
// prevented a scheduled publication from obtaining its next Source opening.
// It retains the original cause for ownership and cleanup decisions.
type sourcePreparationFailure struct {
	stage string
	cause error
}

func (failure *sourcePreparationFailure) Error() string { return failure.cause.Error() }

func (failure *sourcePreparationFailure) Unwrap() error { return failure.cause }

func sourcePreparationFailureAt(stage string, cause error) error {
	return &sourcePreparationFailure{stage: stage, cause: cause}
}

func sourcePreparationFailureStage(cause error) string {
	var failure *sourcePreparationFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

// sourceOperationGate serializes Source opening and issuance across one
// Context lifetime. Its zero value is ready under textContext.mu; the channel
// survives a Source prefix replacement and is never closed on retirement.
type sourceOperationGate struct {
	busy chan struct{}
}

func (gate *sourceOperationGate) initializeLocked() {
	if gate.busy == nil {
		gate.busy = make(chan struct{}, 1)
	}
}

func (gate *sourceOperationGate) acquire(ctx, lease context.Context) (func(), error) {
	select {
	case gate.busy <- struct{}{}:
		if ctx.Err() != nil || lease.Err() != nil {
			<-gate.busy
			return nil, errors.New("text Source operation cancelled")
		}
		return func() { <-gate.busy }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lease.Done():
		return nil, lease.Err()
	}
}

// Serialize actual Source opening/issuance, never a publication ACK or a
// Service stream. Waiters own no tokens and remain cancellable by their caller
// and the independently authorized context. Validation runs after acquisition.
func (owner *textContext) acquireSourceOperation(ctx context.Context) (func(), error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Source operation unavailable")
	}
	owner.mu.Lock()
	if !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.mu.Unlock()
		return nil, errors.New("text Source context unavailable")
	}
	owner.source.operations.initializeLocked()
	gate, lease := &owner.source.operations, owner.lease.Context()
	owner.mu.Unlock()
	return gate.acquire(ctx, lease)
}

// Reconcile the joined Source and reserve its next opening stock during actual
// caller work. No registration, Descriptor ACK or idle timer owns this gate.
func (owner *textContext) prepareSourceReady(ctx context.Context) error {
	release, err := owner.acquireSourceOperation(ctx)
	if err != nil {
		return sourcePreparationFailureAt("operation", err)
	}
	defer release()
	owner.mu.Lock()
	_, _, err = owner.permissionProfileLocked()
	missing := owner.source.currentLocked() == nil
	owner.mu.Unlock()
	if err != nil {
		return sourcePreparationFailureAt("permission", err)
	}
	if missing {
		if _, err := owner.openPrefix(ctx); err != nil {
			return sourcePreparationFailureAt("prefix-"+prefixPreparationFailureStage(err), err)
		}
	}
	return owner.prepareSourceReopenOwned(ctx, nil)
}

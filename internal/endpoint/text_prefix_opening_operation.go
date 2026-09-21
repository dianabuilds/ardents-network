//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// textPrefixOpeningOperation owns the exact stock-to-Source-opening
// reservation and its terminal completion identity. The Source lifecycle
// retains the single admission slot and cancels or joins this operation.
type textPrefixOpeningOperation struct {
	owner           *textContext
	context         context.Context
	cancelOperation context.CancelFunc
	done            chan struct{}
}

func newTextPrefixOpeningOperation(owner *textContext) *textPrefixOpeningOperation {
	attempt, cancel := context.WithCancel(owner.lease.Context())
	return &textPrefixOpeningOperation{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
}

// admittedLocked accepts nil only when no opening owns the context slot. A
// non-nil operation must be the exact live reservation retained by its owner.
func (operation *textPrefixOpeningOperation) admittedLocked(owner *textContext) bool {
	if operation == nil {
		return owner.source.openingAdmittedLocked(nil)
	}
	return operation.owner == owner && owner.source.openingAdmittedLocked(operation) && operation.context.Err() == nil
}

func (operation *textPrefixOpeningOperation) join() {
	if operation != nil {
		<-operation.done
	}
}

func (operation *textPrefixOpeningOperation) cancel() {
	if operation != nil {
		operation.cancelOperation()
	}
}

func (operation *textPrefixOpeningOperation) complete(caller context.Context, prefix *route.ClosedSourcePrefix,
	openErr error) (*textSourceHandle, error) {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(operation.done)
	if !owner.source.openingAdmittedLocked(operation) {
		operation.cancel()
		cleanup := prefix.Close()
		return nil, textPrefixPreparationFailureAt("completion-owner",
			errors.Join(openErr, caller.Err(), cleanup, errors.New("text prefix completion owner changed")))
	}
	if openErr != nil || prefix == nil || caller.Err() != nil || !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.source.finishOpeningLocked(operation, nil, nil, false)
		operation.cancel()
		cleanup := prefix.Close()
		if errors.Is(openErr, route.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, openErr, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		cause := errors.Join(openErr, caller.Err(), cleanup, errors.New("text prefix unavailable"))
		if textPrefixPreparationFailureStage(cause) == "unknown" {
			cause = textPrefixPreparationFailureAt("completion", cause)
		}
		return nil, cause
	}
	handle, current := owner.source.finishOpeningLocked(operation, prefix, operation.cancel, true)
	if !current {
		operation.cancel()
		cleanup := prefix.Close()
		return nil, textPrefixPreparationFailureAt("completion-owner",
			errors.Join(cleanup, errors.New("text prefix completion owner changed")))
	}
	return handle, nil
}

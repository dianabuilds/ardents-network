//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// textOperationFlight records cancellation and completion of one Context
// operation. The Source stock-to-opening reservation, the Publisher
// Introduction/Responder openings and the withdrawal all use this single
// lifetime shape; their separate lifecycle owners retain the pointers.
// For the Source opening the flight also owns the exact stock-to-Source
// reservation and its terminal completion identity: the Source lifecycle
// retains the single admission slot and cancels or joins this operation.
type textOperationFlight struct {
	owner           *textContext
	context         context.Context
	cancelOperation context.CancelFunc
	done            chan struct{}
}

func newTextOperationFlight(owner *textContext) *textOperationFlight {
	attempt, cancel := context.WithCancel(owner.lease.Context())
	return &textOperationFlight{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
}

// admittedLocked accepts nil only when no opening owns the context slot. A
// non-nil operation must be the exact live reservation retained by its owner.
func (flight *textOperationFlight) admittedLocked(owner *textContext) bool {
	if flight == nil {
		return owner.source.openingAdmittedLocked(nil)
	}
	return flight.owner == owner && owner.source.openingAdmittedLocked(flight) && flight.context.Err() == nil
}

func (flight *textOperationFlight) join() {
	if flight != nil {
		<-flight.done
	}
}

func (flight *textOperationFlight) cancel() {
	if flight != nil {
		flight.cancelOperation()
	}
}

// complete terminates the Source opening reservation under owner.mu. Each
// runner completes its flight exactly once; the unconditional done close
// relies on that single-completion discipline.
func (flight *textOperationFlight) complete(caller context.Context, prefix *client.ClosedSourcePrefix,
	openErr error) (*sourceHandle, error) {
	owner := flight.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	if !owner.source.openingAdmittedLocked(flight) {
		flight.cancel()
		cleanup := prefix.Close()
		return nil, prefixPreparationFailureAt("completion-owner",
			errors.Join(openErr, caller.Err(), cleanup, errors.New("text prefix completion owner changed")))
	}
	if openErr != nil || prefix == nil || caller.Err() != nil || !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.source.finishOpeningLocked(flight, nil, nil, false)
		flight.cancel()
		cleanup := prefix.Close()
		if errors.Is(openErr, client.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, openErr, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		cause := errors.Join(openErr, caller.Err(), cleanup, errors.New("text prefix unavailable"))
		if prefixPreparationFailureStage(cause) == "unknown" {
			cause = prefixPreparationFailureAt("completion", cause)
		}
		return nil, cause
	}
	handle, current := owner.source.finishOpeningLocked(flight, prefix, flight.cancel, true)
	if !current {
		flight.cancel()
		cleanup := prefix.Close()
		return nil, prefixPreparationFailureAt("completion-owner",
			errors.Join(cleanup, errors.New("text prefix completion owner changed")))
	}
	return handle, nil
}

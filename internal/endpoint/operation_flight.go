//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// operationFlight records cancellation and completion of one Context
// operation. The Source stock-to-opening reservation, the Publisher
// Introduction/Responder openings and the withdrawal all use this single
// lifetime shape; their separate lifecycle owners retain the pointers.
// For the Source opening the flight also owns the exact stock-to-Source
// reservation and its terminal completion identity: the Source lifecycle
// retains the single admission slot and cancels or joins this operation.
type operationFlight struct {
	owner           *dutyContext
	context         context.Context
	cancelOperation context.CancelFunc
	done            chan struct{}
}

func newOperationFlight(owner *dutyContext) *operationFlight {
	attempt, cancel := context.WithCancel(owner.lease.Context())
	return &operationFlight{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
}

// admittedLocked accepts nil only when no opening owns the context slot. A
// non-nil operation must be the exact live reservation retained by its owner.
func (flight *operationFlight) admittedLocked(owner *dutyContext) bool {
	if flight == nil {
		return owner.source.OpeningAdmittedLocked(nil)
	}
	return flight.owner == owner && owner.source.OpeningAdmittedLocked(flight) && flight.context.Err() == nil
}

func (flight *operationFlight) join() {
	if flight != nil {
		<-flight.done
	}
}

func (flight *operationFlight) cancel() {
	if flight != nil {
		flight.cancelOperation()
	}
}

// CancelFlight and JoinFlight are the source.FlightRef seam: the extracted
// Source lifecycle cancels and joins a retained opening through them without
// depending on the duty context that owns the flight.
func (flight *operationFlight) CancelFlight() { flight.cancel() }

func (flight *operationFlight) JoinFlight() { flight.join() }

// complete terminates the Source opening reservation under owner.mu. Each
// runner completes its flight exactly once; the unconditional done close
// relies on that single-completion discipline.
func (flight *operationFlight) complete(caller context.Context, prefix *client.ClosedSourcePrefix,
	openErr error) (*source.Handle, error) {
	owner := flight.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	if !owner.source.OpeningAdmittedLocked(flight) {
		flight.cancel()
		cleanup := prefix.Close()
		return nil, source.PrefixFailureAt("completion-owner",
			errors.Join(openErr, caller.Err(), cleanup, errors.New("text prefix completion owner changed")))
	}
	if openErr != nil || prefix == nil || caller.Err() != nil || !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.source.FinishOpeningLocked(flight, nil, nil, false)
		flight.cancel()
		cleanup := prefix.Close()
		if errors.Is(openErr, client.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, openErr, cleanup)
			owner.closed = true
			owner.endpoint.failDutyContexts(owner.closeErr)
		}
		cause := errors.Join(openErr, caller.Err(), cleanup, errors.New("text prefix unavailable"))
		if source.PrefixFailureStage(cause) == "unknown" {
			cause = source.PrefixFailureAt("completion", cause)
		}
		return nil, cause
	}
	handle, current := owner.source.FinishOpeningLocked(flight, prefix, flight.cancel, true)
	if !current {
		flight.cancel()
		cleanup := prefix.Close()
		return nil, source.PrefixFailureAt("completion-owner",
			errors.Join(cleanup, errors.New("text prefix completion owner changed")))
	}
	return handle, nil
}

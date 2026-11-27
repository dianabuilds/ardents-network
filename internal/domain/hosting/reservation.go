package hosting

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// ReservationRequest fixes work and cleanup coverage before any durable I/O.
// WorkUntil is the last time a reservation may be transferred for work.
// HoldUntil covers completion; neither date automatically refunds a reservation.
type ReservationRequest struct {
	Work, Termination    Traffic
	WorkUntil, HoldUntil time.Time
}

// Reservation holds admitted work and termination capacity until the
// consumer joins that work and releases it. Losing the handle retains the debit.
type Reservation struct{ *reservationState }

type reservationState struct {
	mu          sync.Mutex
	owner       *Budget
	bytes       uint64
	workBytes   uint64
	released    bool
	err         error
	request     ReservationRequest
	parent      *reservationState
	borrowers   uint64
	sealed      bool
	additionErr error
}

// Reserve commits both work and termination before the caller may admit effects.
// A deadline cannot cross this period; the caller still enforces its independent
// original authority and byte/time limits. Low-watermark space is not lendable.
func (owner *Budget) Reserve(ctx context.Context, request ReservationRequest) (*Reservation, error) {
	if request.WorkUntil.IsZero() || request.HoldUntil.Before(request.WorkUntil) {
		return nil, ErrInvalid
	}
	var reserved, workBytes uint64
	state, _, err := owner.transact(ctx, 0, func(state *hostingState, now time.Time) error {
		view := state.observation(now)
		if view.Protect || view.Drain || !now.Before(request.WorkUntil) || request.HoldUntil.After(state.Policy.End) {
			return ErrCapacity
		}
		workCost, err := state.Policy.cost(request.Work)
		if err != nil {
			return err
		}
		terminalCost, err := state.Policy.cost(request.Termination)
		if err != nil || workCost == 0 || terminalCost == 0 || terminalCost > math.MaxUint64-workCost {
			return ErrInvalid
		}
		reserved = workCost + terminalCost
		workBytes = workCost
		if reserved > view.RemainingBytes || view.RemainingBytes-reserved < state.Policy.LowWatermarkBytes {
			return ErrCapacity
		}
		state.Reserved += reserved
		return nil
	})
	if err != nil {
		return nil, err
	}
	held := &Reservation{reservationState: &reservationState{owner: owner, bytes: reserved, workBytes: workBytes, request: request}}
	transferredAt := owner.now()
	if ctx.Err() != nil || transferredAt.Before(state.Observed) || !transferredAt.Before(request.WorkUntil) {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return nil, errors.Join(ErrCapacity, ctx.Err(), held.Release(cleanup))
	}
	return held, nil
}

// ReleaseJoined releases a reservation after its consumer has joined. It uses
// a separate bounded context so cancellation of the work cannot skip release.
func (reservation *Reservation) ReleaseJoined() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return reservation.Release(ctx)
}

// Release is called only after the consumer has joined its admitted work and
// termination. Actual observed traffic stays spent; an ambiguous release is not
// retried as a second refund against another owner's reservation.
func (reservation *Reservation) Release(ctx context.Context) error {
	if reservation == nil || reservation.reservationState == nil {
		return nil
	}
	if reservation.parent != nil {
		reservation.parent.mu.Lock()
		defer reservation.parent.mu.Unlock()
	}
	reservation.mu.Lock()
	defer reservation.mu.Unlock()
	if reservation.released {
		return reservation.err
	}
	reservation.sealed = true
	if reservation.borrowers != 0 {
		return errors.Join(ErrReservationInUse, reservation.additionErr)
	}
	if reservation.additionErr != nil {
		return reservation.additionErr
	}
	mutating := false
	_, _, reservation.err = reservation.owner.transact(ctx, 0, func(state *hostingState, _ time.Time) error {
		mutating = true
		if reservation.bytes > state.Reserved {
			return errors.New("hosting reservation continuity is unavailable")
		}
		state.Reserved -= reservation.bytes
		return nil
	})
	// A rejected context before this callback has not started a mutation and
	// leaves this handle retryable. Once the callback starts, a later storage
	// failure may follow a committed refund, so the handle remains unresolved.
	reservation.released = mutating
	if reservation.parent != nil && mutating && reservation.err == nil {
		reservation.parent.borrowers--
	}
	if reservation.parent != nil && mutating && reservation.err != nil {
		reservation.parent.sealed = true
		reservation.parent.additionErr = errors.Join(reservation.parent.additionErr, ErrUncertain, reservation.err)
	}
	return reservation.err
}

// DetachTermination transfers the original, already reserved termination
// share to a separate release owner. It creates no durable debit, new traffic
// or time. The work handle can no longer lend termination to additional work.
func (reservation *Reservation) DetachTermination() (func(context.Context) error, error) {
	if reservation == nil || reservation.reservationState == nil {
		return nil, ErrInvalid
	}
	p := reservation.reservationState
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.parent != nil || p.released || p.sealed || p.borrowers != 0 || p.workBytes == 0 || p.workBytes >= p.bytes {
		return nil, ErrUnavailable
	}
	terminal := &Reservation{reservationState: &reservationState{owner: p.owner, bytes: p.bytes - p.workBytes}}
	p.bytes = p.workBytes
	p.request.Termination = Traffic{}
	return terminal.Release, nil
}

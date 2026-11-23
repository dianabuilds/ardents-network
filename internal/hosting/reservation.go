package hosting

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

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

// CoversJoint checks the original work's provider-counted capacity. It does not
// attribute interface counters or certify component bounds or Carrier overhead.
func (owner *Budget) CoversJoint(ctx context.Context, original *Reservation, work JointTraffic) error {
	if owner == nil || original == nil || original.reservationState == nil || ctx == nil {
		return ErrInvalid
	}
	p := original.reservationState
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.owner != owner || p.parent != nil || p.released || p.sealed {
		return ErrUnavailable
	}
	_, _, err := owner.transact(ctx, 0, func(state *hostingState, now time.Time) error {
		if !now.Before(p.request.WorkUntil) {
			return ErrCapacity
		}
		needed, err := work.cost(state.Policy)
		if err != nil {
			return err
		}
		held, err := state.Policy.cost(p.request.Work)
		if err != nil || needed > held {
			return ErrCapacity
		}
		return nil
	})
	return err
}

// ReserveAdditionalJoint reserves an additional shared directional envelope,
// retaining the exact original termination dependency and absolute bounds.
// Release additions after physical join before releasing the original; early
// original Release seals additions and returns ErrReservationInUse without
// returning any capacity.
func (owner *Budget) ReserveAdditionalJoint(ctx context.Context, original *Reservation, work JointTraffic) (*Reservation, error) {
	return owner.reserveAdditional(ctx, original, Traffic{Tx: work.Tx, Rx: work.Rx}, work.cost)
}

func (owner *Budget) reserveAdditional(ctx context.Context, original *Reservation, work Traffic, cost func(Policy) (uint64, error)) (*Reservation, error) {
	if owner == nil || owner.budgetState == nil || ctx == nil || original == nil || original.reservationState == nil {
		return nil, ErrInvalid
	}
	p := original.reservationState
	p.mu.Lock()
	if p.owner != owner || p.parent != nil || p.released || p.sealed || p.request.Termination == (Traffic{}) || p.additionErr != nil || p.borrowers == math.MaxUint64 {
		p.mu.Unlock()
		return nil, ErrUnavailable
	}
	request := p.request
	var reserved uint64
	mutating := false
	state, _, err := owner.transact(ctx, 0, func(state *hostingState, now time.Time) error {
		view := state.observation(now)
		if view.Protect || view.Drain || !now.Before(request.WorkUntil) || request.HoldUntil.After(state.Policy.End) {
			return ErrCapacity
		}
		bytes, err := cost(state.Policy)
		if err != nil || bytes == 0 {
			return ErrInvalid
		}
		if bytes > view.RemainingBytes || view.RemainingBytes-bytes < state.Policy.LowWatermarkBytes {
			return ErrCapacity
		}
		reserved = bytes
		mutating = true
		state.Reserved += bytes
		return nil
	})
	if err != nil {
		if mutating {
			// The durable addition may exist, but no usable handle was handed
			// out. Retain both the uncertainty and its termination dependency.
			p.borrowers++
			p.sealed = true
			p.additionErr = errors.Join(ErrUncertain, err)
		}
		p.mu.Unlock()
		return nil, err
	}
	p.borrowers++
	addition := &Reservation{reservationState: &reservationState{owner: owner, bytes: reserved, parent: p,
		request: ReservationRequest{Work: work, WorkUntil: request.WorkUntil, HoldUntil: request.HoldUntil}}}
	p.mu.Unlock()
	transferredAt := owner.now()
	p.mu.Lock()
	sealed := p.sealed || p.released
	p.mu.Unlock()
	if sealed || ctx.Err() != nil || transferredAt.Before(state.Observed) || !transferredAt.Before(request.WorkUntil) {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return nil, errors.Join(ErrCapacity, ctx.Err(), addition.Release(cleanup))
	}
	return addition, nil
}

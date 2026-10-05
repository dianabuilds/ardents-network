//go:build linux

package hosting

import (
	"context"
	"errors"
	"math"
	"time"
)

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
	if p.owner != owner || p.parent != nil || p.released || p.sealed || p.additionErr != nil || p.borrowers == math.MaxUint64 {
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

package hosting

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"time"
)

// Budget owns access to one shared durable host budget. All local owners must
// use the installation's same root. Opening it never initializes or resets it.
// The runtime adapter reads the named host interfaces, including control,
// retransmission and other processes; these totals are not Application bytes.
type Budget struct{ *budgetState }

type budgetState struct {
	gate     chan struct{}
	root     *os.Root
	measure  func([]string) (hostingReading, error)
	now      func() time.Time
	closed   bool
	closeErr error
}

// Reservation holds admitted work and termination capacity until the
// consumer joins that work and releases it. Losing the handle retains the debit.
type Reservation struct{ *reservationState }

type reservationState struct {
	mu       sync.Mutex
	owner    *Budget
	bytes    uint64
	released bool
	err      error
}

// Initialize explicitly creates a new local period from operator inputs.
// It never resets an existing directory, including an incomplete initialization.
func Initialize(root string, policy Policy) error {
	if err := hostingPlatform(); err != nil {
		return err
	}
	if _, err := policy.limit(); err != nil {
		return errors.Join(ErrInvalid, err)
	}
	reading, err := measureHosting(policy.Interfaces)
	if err != nil {
		return err
	}
	return initializeHosting(root, policy, reading, time.Now().UTC())
}

// Open reopens the same locally installed accounting root. Node or
// Endpoint identities, listeners and job contexts do not select another budget.
func Open(root string) (*Budget, error) {
	if err := hostingPlatform(); err != nil {
		return nil, err
	}
	return openHosting(root, measureHosting, func() time.Time { return time.Now().UTC() })
}

func openHosting(path string, measure func([]string) (hostingReading, error), now func() time.Time) (*Budget, error) {
	if measure == nil || now == nil {
		return nil, errors.New("hosting observation is unavailable")
	}
	root, err := openHostingRoot(path)
	if err != nil {
		return nil, err
	}
	owner := &Budget{&budgetState{gate: make(chan struct{}, 1), root: root, measure: measure, now: now}}
	owner.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := owner.Observe(ctx); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return owner, nil
}

// Observe durably charges the new interface-counter delta before returning a
// pressure decision. Ambiguous counters or storage require drain, never a reset.
func (owner *Budget) Observe(ctx context.Context) (Observation, error) {
	state, observation, err := owner.transact(ctx, 0, nil)
	if err == nil {
		err = ctx.Err()
		if owner.now().Before(state.Observed) {
			err = errors.Join(err, errors.New("hosting observation moved backward at transfer"))
		}
	}
	if err != nil {
		return Observation{Protect: true, Drain: true}, err
	}
	return observation, nil
}

// Reserve commits both work and termination before the caller may admit effects.
// A deadline cannot cross this period; the caller still enforces its independent
// original authority and byte/time limits. Low-watermark space is not lendable.
func (owner *Budget) Reserve(ctx context.Context, request ReservationRequest) (*Reservation, error) {
	if request.WorkUntil.IsZero() || request.HoldUntil.Before(request.WorkUntil) {
		return nil, ErrInvalid
	}
	var reserved uint64
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
		if reserved > view.RemainingBytes || view.RemainingBytes-reserved < state.Policy.LowWatermarkBytes {
			return ErrCapacity
		}
		state.Reserved += reserved
		return nil
	})
	if err != nil {
		return nil, err
	}
	held := &Reservation{reservationState: &reservationState{owner: owner, bytes: reserved}}
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
	reservation.mu.Lock()
	defer reservation.mu.Unlock()
	if reservation.released {
		return reservation.err
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
	return reservation.err
}

// Close releases this handle only. Unreleased work reservations remain durable.
func (owner *Budget) Close() error {
	if owner == nil || owner.budgetState == nil {
		return nil
	}
	<-owner.gate
	defer func() { owner.gate <- struct{}{} }()
	if !owner.closed {
		owner.closed = true
		owner.closeErr = owner.root.Close()
	}
	return owner.closeErr
}

func (owner *Budget) transact(ctx context.Context, maximumAge time.Duration,
	change func(*hostingState, time.Time) error) (hostingState, Observation, error) {
	var empty hostingState
	unavailable := Observation{Protect: true, Drain: true}
	if owner == nil || owner.budgetState == nil || ctx == nil || ctx.Err() != nil {
		if ctx != nil && ctx.Err() != nil {
			return empty, unavailable, ctx.Err()
		}
		return empty, unavailable, errors.New("hosting operation is unavailable")
	}
	if err := owner.enter(ctx); err != nil {
		return empty, unavailable, err
	}
	defer owner.leave()
	if owner.closed || ctx.Err() != nil {
		if ctx.Err() != nil {
			return empty, unavailable, ctx.Err()
		}
		return empty, unavailable, errors.New("hosting owner is unavailable")
	}
	lease, err := acquireHostingLease(ctx, owner.root)
	if err != nil {
		return empty, unavailable, err
	}
	state, err := readHostingState(owner.root)
	if err != nil {
		return empty, unavailable, errors.Join(err, lease.close())
	}
	now := owner.now()
	if now.Before(state.Observed) {
		return empty, unavailable, errors.Join(errors.New("hosting observation moved backward"), lease.close())
	}
	recent := maximumAge > 0 && now.Sub(state.Observed) <= maximumAge
	if !recent {
		reading, measureErr := owner.measure(state.Policy.Interfaces)
		err = measureErr
		if err == nil {
			err = state.observe(reading, now)
		}
		if err != nil {
			return empty, unavailable, errors.Join(err, lease.close())
		}
	}
	var refusal error
	if ctx.Err() != nil {
		refusal = ctx.Err()
	} else if change != nil {
		current := owner.now()
		if current.Before(now) {
			refusal = errors.New("hosting clock moved backward during observation")
		} else {
			refusal = change(&state, current)
		}
	}
	// Even a refused reservation must retain already incurred host traffic.
	if !recent || change != nil {
		if err := writeHostingState(owner.root, state); err != nil {
			return empty, unavailable, errors.Join(ErrUncertain, err, lease.close())
		}
	}
	if err := lease.close(); err != nil {
		return empty, unavailable, errors.Join(ErrUncertain, refusal, err)
	}
	return state, state.observation(owner.now()), refusal
}

func (owner *Budget) enter(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-owner.gate:
		return nil
	}
}

func (owner *Budget) leave() { owner.gate <- struct{}{} }

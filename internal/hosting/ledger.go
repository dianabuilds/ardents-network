package hosting

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"time"
)

// Ledger owns access to one shared durable host period. All local owners must
// use the installation's same root. Opening it never initializes or resets it.
// The runtime adapter reads the named host interfaces, including control,
// retransmission and other processes; these totals are not Application bytes.
type Ledger struct {
	gate     chan struct{}
	root     *os.Root
	measure  func([]string) (hostingReading, error)
	now      func() time.Time
	closed   bool
	closeErr error
}

// Reservation holds admitted work and termination capacity until the
// consumer joins that work and releases it. Losing the handle retains the debit.
type Reservation struct {
	state *reservation
}

// reservation is shared across copies of the public handle. Only this owner
// may decide whether release has started, committed, or become uncertain.
type reservation struct {
	mu                sync.Mutex
	owner             *Ledger
	bytes             uint64
	released          bool
	err               error
	invalidateFailure func()
}

// Initialize explicitly creates a new local period from operator inputs.
// It never resets an existing directory, including an incomplete initialization.
func Initialize(root string, policy Policy) error {
	if err := hostingPlatform(); err != nil {
		return err
	}
	if _, err := policy.limit(); err != nil {
		return err
	}
	reading, err := measureHosting(policy.Interfaces)
	if err != nil {
		return err
	}
	return initializeHosting(root, policy, reading, time.Now().UTC())
}

// Open reopens the same locally installed accounting root. Node or
// Endpoint identities, listeners and job contexts do not select another period.
func Open(root string) (*Ledger, error) {
	if err := hostingPlatform(); err != nil {
		return nil, err
	}
	return openHosting(root, measureHosting, func() time.Time { return time.Now().UTC() })
}

func openHosting(path string, measure func([]string) (hostingReading, error), now func() time.Time) (*Ledger, error) {
	if measure == nil || now == nil {
		return nil, errors.New("hosting observation is unavailable")
	}
	root, err := openHostingRoot(path)
	if err != nil {
		return nil, err
	}
	owner := &Ledger{gate: make(chan struct{}, 1), root: root, measure: measure, now: now}
	owner.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := owner.Sample(ctx, time.Second); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return owner, nil
}

// Observe durably charges the new interface-counter delta before returning a
// pressure decision. Ambiguous counters or storage require drain, never a reset.
func (owner *Ledger) Observe(ctx context.Context) (Observation, error) {
	_, observation, err := owner.transact(ctx, 0, nil)
	return observation, err
}

// Reserve commits both work and termination before the caller may admit effects.
// A deadline cannot cross this period; the caller still enforces its independent
// original authority and byte/time limits. Low-watermark space is not lendable.
func (owner *Ledger) Reserve(ctx context.Context, work, termination Traffic, end time.Time) (*Reservation, error) {
	var reserved uint64
	_, _, err := owner.transact(ctx, 0, func(state *hostingState, now time.Time) error {
		view := state.observation(now)
		if view.Protect || view.Drain || !now.Before(end) || end.After(state.Policy.End) {
			return errors.New("hosting admission is unavailable")
		}
		workCost, err := state.Policy.cost(work)
		if err != nil {
			return err
		}
		terminalCost, err := state.Policy.cost(termination)
		if err != nil || workCost == 0 || terminalCost == 0 || terminalCost > math.MaxUint64-workCost {
			return errors.New("hosting reservation is invalid")
		}
		reserved = workCost + terminalCost
		if reserved > view.RemainingBytes || view.RemainingBytes-reserved < state.Policy.LowWatermarkBytes {
			return errors.New("hosting allowance cannot reserve work and termination")
		}
		state.Reserved += reserved
		return nil
	})
	if err != nil {
		return nil, err
	}
	handle := &Reservation{state: &reservation{owner: owner, bytes: reserved}}
	// Slow measurement and durable I/O cannot extend the caller's deadline.
	// Cleanup has its own bounded context because the request may be canceled.
	if ctx.Err() != nil || !owner.now().Before(end) {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return nil, errors.Join(errors.New("hosting reservation expired before transfer"), ctx.Err(), handle.Release(cleanup))
	}
	return handle, nil
}

// Release is called only after the consumer has joined its admitted work and
// termination. Actual observed traffic stays spent; an ambiguous release is not
// retried as a second refund against another owner's reservation.
func (handle *Reservation) Release(ctx context.Context) error {
	if handle == nil || handle.state == nil {
		return nil
	}
	reservation := handle.state
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
	if reservation.err != nil && reservation.invalidateFailure != nil {
		reservation.invalidateFailure()
	}
	return reservation.err
}

// Close releases this handle only. Unreleased work reservations remain durable.
func (owner *Ledger) Close() error {
	if owner == nil {
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

func (owner *Ledger) transact(ctx context.Context, maximumAge time.Duration,
	change func(*hostingState, time.Time) error) (hostingState, Observation, error) {
	var empty hostingState
	unavailable := Observation{Protect: true, Drain: true}
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return empty, unavailable, errors.New("hosting operation is unavailable")
	}
	if err := owner.enter(ctx); err != nil {
		return empty, unavailable, err
	}
	defer owner.leave()
	if owner.closed || ctx.Err() != nil {
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
	recent := maximumAge > 0 && !now.Before(state.Observed) && now.Sub(state.Observed) <= maximumAge
	if maximumAge > 0 && now.Before(state.Observed) {
		return empty, unavailable, errors.Join(errors.New("hosting observation continuity is unavailable"), lease.close())
	}
	if !recent {
		reading, measureErr := owner.measure(state.Policy.Interfaces)
		err = measureErr
		if err == nil {
			err = state.observe(reading, now)
		}
	}
	if err != nil {
		return empty, unavailable, errors.Join(err, lease.close())
	}
	var refusal error
	if ctx.Err() != nil {
		refusal = ctx.Err()
	} else if change != nil {
		refusal = change(&state, owner.now())
	}
	// Even a refused reservation must retain already incurred host traffic.
	if !recent || change != nil {
		if err := writeHostingState(owner.root, state); err != nil {
			return empty, unavailable, errors.Join(err, lease.close())
		}
	}
	if err := lease.close(); err != nil {
		return empty, unavailable, errors.Join(refusal, err)
	}
	return state, state.observation(now), refusal
}

func (owner *Ledger) enter(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-owner.gate:
		return nil
	}
}

func (owner *Ledger) leave() { owner.gate <- struct{}{} }

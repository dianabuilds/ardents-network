package resource

import (
	"context"
	"errors"
	"math"
	"os"
	"sync"
	"time"
)

// Hosting owns access to one shared durable host period. All local owners must
// use the installation's same root. Opening it never initializes or resets it.
// The runtime adapter reads the named host interfaces, including control,
// retransmission and other processes; these totals are not Application bytes.
type Hosting struct {
	gate     chan struct{}
	root     *os.Root
	measure  func([]string) (hostingReading, error)
	now      func() time.Time
	closed   bool
	closeErr error
}

// HostingReservation holds admitted work and termination capacity until the
// consumer joins that work and releases it. Losing the handle retains the debit.
type HostingReservation struct {
	mu       sync.Mutex
	owner    *Hosting
	bytes    uint64
	released bool
	err      error
}

// InitializeHosting explicitly creates a new local period from operator inputs.
// It never resets an existing directory, including an incomplete initialization.
func InitializeHosting(root string, policy HostingPolicy) error {
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

// OpenHosting reopens the same locally installed accounting root. Node or
// Endpoint identities, listeners and job contexts do not select another period.
func OpenHosting(root string) (*Hosting, error) {
	if err := hostingPlatform(); err != nil {
		return nil, err
	}
	return openHosting(root, measureHosting, func() time.Time { return time.Now().UTC() })
}

func openHosting(path string, measure func([]string) (hostingReading, error), now func() time.Time) (*Hosting, error) {
	if measure == nil || now == nil {
		return nil, errors.New("hosting observation is unavailable")
	}
	root, err := openHostingRoot(path)
	if err != nil {
		return nil, err
	}
	owner := &Hosting{gate: make(chan struct{}, 1), root: root, measure: measure, now: now}
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
func (owner *Hosting) Observe(ctx context.Context) (HostingObservation, error) {
	_, observation, err := owner.transact(ctx, 0, nil)
	return observation, err
}

// Reserve commits both work and termination before the caller may admit effects.
// A deadline cannot cross this period; the caller still enforces its independent
// original authority and byte/time limits. Low-watermark space is not lendable.
func (owner *Hosting) Reserve(ctx context.Context, work, termination HostingTraffic, end time.Time) (*HostingReservation, error) {
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
	return &HostingReservation{owner: owner, bytes: reserved}, nil
}

// Release is called only after the consumer has joined its admitted work and
// termination. Actual observed traffic stays spent; an ambiguous release is not
// retried as a second refund against another owner's reservation.
func (reservation *HostingReservation) Release(ctx context.Context) error {
	if reservation == nil {
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
func (owner *Hosting) Close() error {
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

func (owner *Hosting) transact(ctx context.Context, maximumAge time.Duration,
	change func(*hostingState, time.Time) error) (hostingState, HostingObservation, error) {
	var empty hostingState
	unavailable := HostingObservation{Protect: true, Drain: true}
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
		refusal = change(&state, now)
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

func (owner *Hosting) enter(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-owner.gate:
		return nil
	}
}

func (owner *Hosting) leave() { owner.gate <- struct{}{} }

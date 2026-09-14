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
	mu       sync.Mutex
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
	owner := &Hosting{root: root, measure: measure, now: now}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := owner.Observe(ctx); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return owner, nil
}

// Observe durably charges the new interface-counter delta before returning a
// pressure decision. Ambiguous counters or storage require drain, never a reset.
func (owner *Hosting) Observe(ctx context.Context) (HostingObservation, error) {
	return owner.transact(ctx, nil)
}

// Reserve commits both work and termination before the caller may admit effects.
// A deadline cannot cross this period; the caller still enforces its independent
// original authority and byte/time limits. Low-watermark space is not lendable.
func (owner *Hosting) Reserve(ctx context.Context, work, termination HostingTraffic, end time.Time) (*HostingReservation, error) {
	var reserved uint64
	_, err := owner.transact(ctx, func(state *hostingState, now time.Time) error {
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
	reservation.released = true
	_, reservation.err = reservation.owner.transact(ctx, func(state *hostingState, _ time.Time) error {
		if reservation.bytes > state.Reserved {
			return errors.New("hosting reservation continuity is unavailable")
		}
		state.Reserved -= reservation.bytes
		return nil
	})
	return reservation.err
}

// Close releases this handle only. Unreleased work reservations remain durable.
func (owner *Hosting) Close() error {
	if owner == nil {
		return nil
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.closed {
		owner.closed = true
		owner.closeErr = owner.root.Close()
	}
	return owner.closeErr
}

func (owner *Hosting) transact(ctx context.Context, change func(*hostingState, time.Time) error) (HostingObservation, error) {
	unavailable := HostingObservation{Protect: true, Drain: true}
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return unavailable, errors.New("hosting operation is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || ctx.Err() != nil {
		return unavailable, errors.New("hosting owner is unavailable")
	}
	lease, err := acquireHostingLease(ctx, owner.root)
	if err != nil {
		return unavailable, err
	}
	state, err := readHostingState(owner.root)
	if err != nil {
		return unavailable, errors.Join(err, lease.close())
	}
	reading, err := owner.measure(state.Policy.Interfaces)
	now := owner.now()
	if err == nil {
		err = state.observe(reading, now)
	}
	if err != nil {
		return unavailable, errors.Join(err, lease.close())
	}
	var refusal error
	if ctx.Err() != nil {
		refusal = ctx.Err()
	} else if change != nil {
		refusal = change(&state, now)
	}
	// Even a refused reservation must retain already incurred host traffic.
	if err := writeHostingState(owner.root, state); err != nil {
		return unavailable, errors.Join(err, lease.close())
	}
	if err := lease.close(); err != nil {
		return unavailable, errors.Join(refusal, err)
	}
	return state.observation(now), refusal
}

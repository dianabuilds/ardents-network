package hosting

import (
	"context"
	"errors"
	"os"
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

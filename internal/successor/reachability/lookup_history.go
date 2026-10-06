package reachability

import (
	"context"
	"errors"
	"sync"
	"time"
)

// History owns the private monotonic facts of one exact live context. Worker
// loss does not retire it. Close retires the context, interrupts the retained
// lookup and waits for its caller's physical join before clearing the facts.
// It is distinct from receiving Store and retains no signed proof bytes.
type History struct {
	mu      sync.Mutex
	floors  map[[32]byte]historyFloor
	active  *Lookup
	retired bool
}

// Lookup retains one original serialized flight, independently selected proof
// bindings and a synchronous original Source/lifetime guard. Context interrupts
// dependent I/O; Finish is called only after its physical resources have joined.
type Lookup struct {
	owner                    *History
	target, network, profile [32]byte
	ctx                      context.Context
	cancel                   context.CancelFunc
	check                    func() error
	done                     chan struct{}
	completed                bool // guarded by owner.mu
	once                     sync.Once
}

// Begin refuses capacity before issuance/admission. There is no waiting queue
// or replacement flight: a busy context refuses until its original flight joins.
// check must retain the actual original Source and never call into History.
// It runs outside History's mutex so observing genuine neighboring authority
// cannot block local retirement. It supplies no authenticity by itself.
func (history *History) Begin(caller context.Context, target, network, profile [32]byte, check func() error) (*Lookup, error) {
	if history == nil || caller == nil || check == nil || target == [32]byte{} || network == [32]byte{} || profile == [32]byte{} {
		return nil, errors.New("reachability lookup lifetime or binding invalid")
	}
	history.mu.Lock()
	err := history.canBeginLocked(target)
	history.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := errors.Join(caller.Err(), check()); err != nil {
		return nil, err
	}
	history.mu.Lock()
	defer history.mu.Unlock()
	if err := errors.Join(history.canBeginLocked(target), caller.Err()); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(caller)
	flight := &Lookup{owner: history, target: target, network: network, profile: profile,
		ctx: ctx, cancel: cancel, check: check, done: make(chan struct{})}
	history.active = flight
	return flight, nil
}

func (history *History) canBeginLocked(target [32]byte) error {
	if history.retired || history.active != nil {
		return errors.New("reachability lookup context retired or busy")
	}
	if _, exists := history.floors[target]; !exists && len(history.floors) >= MaximumTargets {
		return errors.New("reachability lookup Target capacity exhausted")
	}
	return nil
}

func (lookup *Lookup) Context() context.Context { return lookup.ctx }

// Complete verifies raw proof independently, then rechecks the exact flight
// after verification and before its floor/final handoff. Route invokes it after
// the actual exchange and physical join, while the original Source is retained.
// A successful proof return grants neither Instance authentication nor readiness.
func (lookup *Lookup) Complete(raw []byte, at time.Time) (Descriptor, error) {
	if lookup == nil || lookup.owner == nil {
		return Descriptor{}, errors.New("reachability original lookup absent")
	}
	history := lookup.owner
	if err := lookup.current(); err != nil {
		return Descriptor{}, err
	}
	proof, err := Verify(raw, lookup.target, lookup.network, lookup.profile, at)
	if err != nil {
		return Descriptor{}, err
	}
	if err := lookup.current(); err != nil {
		return Descriptor{}, err
	}
	history.mu.Lock()
	if err := lookup.currentLocked(); err != nil {
		history.mu.Unlock()
		return Descriptor{}, err
	}
	lookup.completed = true
	candidate := descriptorFloor(proof)
	if prior, exists := history.floors[lookup.target]; exists {
		_, next, err := compareHistoryFloor(prior, candidate)
		if next != nil {
			history.floors[lookup.target] = *next
		}
		if err != nil {
			history.mu.Unlock()
			return Descriptor{}, err
		}
	} else {
		if history.floors == nil {
			history.floors = make(map[[32]byte]historyFloor)
		}
		history.floors[lookup.target] = candidate
	}
	history.mu.Unlock()
	// Guard failure denies export while keeping the monotonic facts already
	// observed under the original flight. It never revives an older Descriptor.
	if err := errors.Join(lookup.ctx.Err(), lookup.check()); err != nil {
		return Descriptor{}, err
	}
	history.mu.Lock()
	defer history.mu.Unlock()
	if err := lookup.liveLocked(); err != nil {
		return Descriptor{}, err
	}
	return proof, nil
}

func (lookup *Lookup) currentLocked() error {
	if lookup.completed {
		return errors.New("reachability original lookup unavailable")
	}
	return lookup.liveLocked()
}

func (lookup *Lookup) liveLocked() error {
	if lookup.owner.retired || lookup.owner.active != lookup {
		return errors.New("reachability original lookup unavailable")
	}
	return lookup.ctx.Err()
}

func (lookup *Lookup) current() error {
	lookup.owner.mu.Lock()
	err := lookup.currentLocked()
	lookup.owner.mu.Unlock()
	if err != nil {
		return err
	}
	err = errors.Join(lookup.ctx.Err(), lookup.check())
	lookup.owner.mu.Lock()
	defer lookup.owner.mu.Unlock()
	return errors.Join(err, lookup.currentLocked())
}

// Finish releases this flight after physical join. Repeated calls cannot release
// a later flight or clear its facts; unsuccessful lookup retains existing floors.
func (lookup *Lookup) Finish() {
	if lookup == nil || lookup.owner == nil {
		return
	}
	lookup.once.Do(func() {
		lookup.cancel()
		lookup.owner.mu.Lock()
		defer lookup.owner.mu.Unlock()
		if lookup.owner.active == lookup {
			lookup.owner.active = nil
		}
		close(lookup.done)
	})
}

func (history *History) Close() error {
	if history == nil {
		return nil
	}
	history.mu.Lock()
	history.retired = true
	active := history.active
	if active != nil {
		active.cancel()
	}
	history.mu.Unlock()
	if active != nil {
		<-active.done
	}
	history.mu.Lock()
	clear(history.floors)
	history.floors = nil
	history.mu.Unlock()
	return nil
}

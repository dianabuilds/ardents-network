package introduction

import (
	"errors"
	"sync"
	"time"
)

// Registry serializes live channel ownership with durable non-reclaim claims.
// Its caller authenticates the role channel and supplies the original accepted
// allowance; this owner cannot verify tokens or declare Publication readiness.
type Registry struct {
	mu      sync.Mutex
	history *History
	slots   map[[32]byte]*Registration
	closed  bool
	pending uint64
}

// Capacity is one reserved registration position. Only Registry can create it;
// it transfers to a claim once and returns only after physical channel join.
type Capacity struct {
	registry            *Registry
	committed, released bool
}

// Reserve checks retained non-reclaim occupancy before Admission spending.
// It does not prune or write history and cannot erase an earlier time floor.
func (registry *Registry) Reserve(now time.Time) (*Capacity, error) {
	if registry == nil || now.IsZero() {
		return nil, errors.New("introduction capacity unavailable")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.closed {
		return nil, errors.New("introduction registry closed")
	}
	if err := registry.history.checkCapacity(now, registry.pending); err != nil {
		return nil, err
	}
	registry.pending++
	return &Capacity{registry: registry}, nil
}

// Release is idempotent. A committed position remains in durable history;
// returning physical capacity never refunds its original non-reclaim expiry.
func (capacity *Capacity) Release() {
	if capacity == nil || capacity.registry == nil {
		return
	}
	registry := capacity.registry
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if !capacity.released {
		capacity.released = true
		if !capacity.committed {
			registry.pending--
		}
	}
}

// Registration belongs to one exact TLS channel and immutable original expiry.
// Retiring it removes live delivery authority without releasing physical work
// or erasing the durable non-reclaim floor.
type Registration struct {
	registry                         *Registry
	request                          Request
	channel                          [32]byte
	used, maximum                    uint64
	acknowledged, retired, withdrawn bool
	dispatch                         *registrationDispatch
}

// NewRegistry transfers an already durable, independently owned Route history
// to its sole live capacity owner. A second registry cannot get another pending
// counter or close the original owner's root. Reopen requires a new leased History.
func NewRegistry(history *History) (*Registry, error) {
	if history == nil {
		return nil, errors.New("introduction history absent")
	}
	if err := history.claimRegistry(); err != nil {
		return nil, err
	}
	registry := &Registry{history: history, slots: make(map[[32]byte]*Registration)}
	return registry, nil
}

// Register reserves both complete fixed exchanges before claiming history.
// used counts already exchanged ingress and egress including frame headers.
// deadline and maximum belong to the genuine original Admission Grant.
func (registry *Registry) Register(capacity *Capacity, request Request, channel [32]byte, maximum uint64, deadline time.Time, used uint64, now time.Time) (*Registration, error) {
	if registry == nil || request.Withdraw || request.Nonce == [32]byte{} || request.Slot == [32]byte{} || request.Revision == 0 || channel == [32]byte{} || now.IsZero() || !now.Before(request.Expiry) || request.Expiry.After(deadline) || request.Expiry.After(now.Add(600*time.Second)) {
		return nil, errors.New("introduction registration invalid")
	}
	// Register operation/result plus owning withdraw operation/result. No
	// delivery may borrow this reserve; no Admission class policy is duplicated.
	const exchanges = registrationExchangeBytes
	if used > maximum || exchanges > maximum-used {
		return nil, errors.New("introduction registration withdrawal reserve exhausted")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.closed {
		return nil, errors.New("introduction registry closed")
	}
	if capacity == nil || capacity.registry != registry || capacity.released || capacity.committed {
		return nil, errors.New("introduction registration capacity absent")
	}
	for slot, prior := range registry.slots {
		if !now.Before(prior.request.Expiry) {
			delete(registry.slots, slot)
		}
	}
	if _, found := registry.slots[request.Slot]; found || len(registry.slots) >= slotMaximum {
		return nil, errors.New("introduction slot unavailable")
	}
	if err := registry.history.Claim(request.Slot, request.Expiry, now); err != nil {
		return nil, err
	}
	capacity.committed = true
	registry.pending--
	registration := &Registration{registry: registry, request: request, channel: channel, used: used + exchanges, maximum: maximum}
	registry.slots[request.Slot] = registration
	return registration, nil
}

// Acknowledge enables the exact slot only after its actual complete RESULT.
// The transport checks current authenticated duty again around that write.
func (registration *Registration) Acknowledge(now time.Time) error {
	if registration == nil || registration.registry == nil {
		return errors.New("introduction registration absent")
	}
	registry := registration.registry
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.closed || registration.retired || registration.acknowledged || !now.Before(registration.request.Expiry) || registry.slots[registration.request.Slot] != registration {
		return errors.New("introduction registration acknowledgement unavailable")
	}
	registration.acknowledged = true
	return nil
}

// Withdraw requires a fresh nonce and exact slot/revision on the owning channel.
// It retires transport authority; physical join remains with the work owner.
func (registration *Registration) Withdraw(request Request, channel [32]byte, now time.Time) error {
	if registration == nil || registration.registry == nil {
		return errors.New("introduction registration absent")
	}
	registry := registration.registry
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.closed || registration.retired || !registration.acknowledged || registration.withdrawn || !now.Before(registration.request.Expiry) || registry.slots[registration.request.Slot] != registration ||
		!request.Withdraw || request.Nonce == [32]byte{} || request.Nonce == registration.request.Nonce || request.Slot != registration.request.Slot || request.Revision != registration.request.Revision || !request.Expiry.IsZero() || channel != registration.channel {
		return errors.New("introduction owning withdrawal unavailable")
	}
	registration.withdrawn, registration.retired = true, true
	return nil
}

// Retire is idempotent. A failed or lost ACK retains original occupancy/history.
func (registration *Registration) Retire() {
	if registration == nil || registration.registry == nil {
		return
	}
	registry := registration.registry
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registration.retired = true
}

// Close is called only after the receiving work owner has stopped and joined
// its physical borrowers. It returns the history's retained terminal outcome.
func (registry *Registry) Close() error {
	if registry == nil {
		return nil
	}
	registry.mu.Lock()
	registry.closed = true
	for _, registration := range registry.slots {
		registration.retired = true
	}
	registry.mu.Unlock()
	return registry.history.Close()
}

package route

import (
	"errors"
	"sync"
	"time"
)

const (
	closedCarrierPoolMaximum = 32
	closedCarrierRetention   = 120 * time.Second
)

// ClosedCarrierKey binds a retained actual-work Carrier to the exact current
// public State facts. It has no Target, Endpoint identity or fallback field.
type ClosedCarrierKey struct {
	NetworkID, ProfileDigest, LocalNodeID, PeerNodeID, PeerKey [32]byte
	CarrierProfile                                             CarrierProfile
}

// ClosedCarrierPool retains only useful closed Node Carriers. The State owner
// supplies validate on every acquisition and invalidates changed facts; this
// pool never independently selects, dials or retries a peer.
type ClosedCarrierPool struct {
	mu      sync.Mutex
	clock   func() time.Time
	entries map[ClosedCarrierKey]*closedCarrierEntry
}

type closedCarrierEntry struct {
	carrier Carrier
	active  uint16
	used    bool
	idleAt  time.Time
}

// ClosedCarrierLease is one joined child owner. MarkUsed is required before a
// released carrier may remain in the finite pool.
type ClosedCarrierLease struct {
	pool     *ClosedCarrierPool
	key      ClosedCarrierKey
	entry    *closedCarrierEntry
	used     bool
	released bool
}

// NewClosedCarrierPool creates one local Node pool with no pre-dial entries.
func NewClosedCarrierPool(clock func() time.Time) (*ClosedCarrierPool, error) {
	if clock == nil || clock().IsZero() {
		return nil, errors.New("closed Carrier pool clock is invalid")
	}
	return &ClosedCarrierPool{clock: clock, entries: make(map[ClosedCarrierKey]*closedCarrierEntry)}, nil
}

// Acquire validates current State facts before using a retained Carrier or
// calling open. The caller supplies the one exact State-selected dial attempt;
// open is never called to refill an idle pool.
func (pool *ClosedCarrierPool) Acquire(key ClosedCarrierKey, validate func() error, open func() (Carrier, error)) (*ClosedCarrierLease, error) {
	if pool == nil || !validClosedCarrierKey(key) || validate == nil || open == nil || validate() != nil {
		return nil, errors.New("closed Carrier acquisition is unavailable")
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	now := pool.clock().UTC()
	pool.reapLocked(now)
	if entry := pool.entries[key]; entry != nil {
		entry.active++
		entry.idleAt = time.Time{}
		return &ClosedCarrierLease{pool: pool, key: key, entry: entry}, nil
	}
	if len(pool.entries) >= closedCarrierPoolMaximum {
		return nil, errors.New("closed Carrier pool is exhausted")
	}
	carrier, err := open()
	if err != nil || carrier == nil {
		return nil, errors.New("closed Carrier acquisition is unavailable")
	}
	entry := &closedCarrierEntry{carrier: carrier, active: 1}
	pool.entries[key] = entry
	return &ClosedCarrierLease{pool: pool, key: key, entry: entry}, nil
}

// MarkUsed records that this lease carried actual work. An opened but unused
// Carrier closes at Release instead of becoming speculative readiness.
func (lease *ClosedCarrierLease) MarkUsed() error {
	if lease == nil || lease.pool == nil {
		return errors.New("closed Carrier lease is unavailable")
	}
	lease.pool.mu.Lock()
	defer lease.pool.mu.Unlock()
	if lease.released || lease.pool.entries[lease.key] != lease.entry {
		return errors.New("closed Carrier lease is unavailable")
	}
	lease.used, lease.entry.used = true, true
	return nil
}

// Release joins one child. The last useful child starts the fixed retention;
// the last unused child closes immediately.
func (lease *ClosedCarrierLease) Release() error {
	if lease == nil || lease.pool == nil {
		return nil
	}
	lease.pool.mu.Lock()
	defer lease.pool.mu.Unlock()
	if lease.released {
		return nil
	}
	lease.released = true
	if lease.pool.entries[lease.key] != lease.entry || lease.entry.active == 0 {
		return nil
	}
	lease.entry.active--
	if lease.entry.active > 0 {
		return nil
	}
	if !lease.entry.used {
		delete(lease.pool.entries, lease.key)
		return lease.entry.carrier.Close()
	}
	lease.entry.idleAt = lease.pool.clock().UTC()
	return nil
}

// Invalidate closes a changed State key even while children are active. Their
// owned readers/writers observe the closed Carrier and must join normally.
func (pool *ClosedCarrierPool) Invalidate(key ClosedCarrierKey) error {
	if pool == nil {
		return nil
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	entry := pool.entries[key]
	if entry == nil {
		return nil
	}
	delete(pool.entries, key)
	return entry.carrier.Close()
}

// Reap closes only idle, previously useful Carriers past the fixed retention.
func (pool *ClosedCarrierPool) Reap() error {
	if pool == nil {
		return nil
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.reapLocked(pool.clock().UTC())
}

// Close releases every retained or active Carrier during Node withdrawal.
func (pool *ClosedCarrierPool) Close() error {
	if pool == nil {
		return nil
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	var result error
	for key, entry := range pool.entries {
		delete(pool.entries, key)
		result = errors.Join(result, entry.carrier.Close())
	}
	return result
}

func (pool *ClosedCarrierPool) reapLocked(now time.Time) error {
	var result error
	for key, entry := range pool.entries {
		if entry.active == 0 && !entry.idleAt.IsZero() && !now.Before(entry.idleAt.Add(closedCarrierRetention)) {
			delete(pool.entries, key)
			result = errors.Join(result, entry.carrier.Close())
		}
	}
	return result
}

func validClosedCarrierKey(key ClosedCarrierKey) bool {
	return key.NetworkID != [32]byte{} && key.ProfileDigest != [32]byte{} && key.LocalNodeID != [32]byte{} && key.PeerNodeID != [32]byte{} &&
		key.PeerKey != [32]byte{} && key.LocalNodeID != key.PeerNodeID && (key.CarrierProfile == ClosedCarrierTCP || key.CarrierProfile == ClosedCarrierQUIC)
}

package route

import (
	"context"
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
	mu         sync.Mutex
	clock      func() time.Time
	entries    map[ClosedCarrierKey]*closedCarrierEntry
	operations map[closedCarrierPair]*closedCarrierOperation
	closed     bool
	closeErr   error
}

type closedCarrierPair struct{ local, peer [32]byte }

type closedCarrierOperation struct{ done chan struct{} }

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

// Carrier returns the one State-validated Carrier owned by this live lease.
// Callers may use it only while they retain the lease; Release closes an
// unused Carrier or returns actual work to the bounded pool.
func (lease *ClosedCarrierLease) Carrier() (Carrier, error) {
	if lease == nil || lease.pool == nil {
		return nil, errors.New("closed Carrier lease is unavailable")
	}
	lease.pool.mu.Lock()
	defer lease.pool.mu.Unlock()
	if lease.released || lease.pool.closed || lease.pool.entries[lease.key] != lease.entry || lease.entry.carrier == nil {
		return nil, errors.New("closed Carrier lease is unavailable")
	}
	return lease.entry.carrier, nil
}

// NewClosedCarrierPool creates one local Node pool with no pre-dial entries.
func NewClosedCarrierPool(clock func() time.Time) (*ClosedCarrierPool, error) {
	if clock == nil || clock().IsZero() {
		return nil, errors.New("closed Carrier pool clock is invalid")
	}
	return &ClosedCarrierPool{clock: clock, entries: make(map[ClosedCarrierKey]*closedCarrierEntry), operations: make(map[closedCarrierPair]*closedCarrierOperation)}, nil
}

// AcquireContext waits for an existing dial of this exact key without blocking
// unrelated directed pairs. The caller's context bounds that wait and dial; it
// does not create a retry or a speculative dial.
func (pool *ClosedCarrierPool) AcquireContext(ctx context.Context, key ClosedCarrierKey, validate func() error, open func() (Carrier, error)) (*ClosedCarrierLease, error) {
	if pool == nil || ctx == nil || !validClosedCarrierKey(key) || validate == nil || open == nil {
		return nil, errors.New("closed Carrier acquisition is unavailable")
	}
	for {
		// A caller can wait behind another exact-key dial. Recheck before
		// borrowing its result, and before starting any replacement dial.
		if ctx.Err() != nil || validate() != nil {
			return nil, errors.New("closed Carrier acquisition is unavailable")
		}
		pool.mu.Lock()
		if pool.closed {
			pool.mu.Unlock()
			return nil, errors.New("closed Carrier pool is closed")
		}
		pair := key.pair()
		if operation := pool.operations[pair]; operation != nil {
			pool.mu.Unlock()
			select {
			case <-operation.done:
				continue
			case <-ctx.Done():
				return nil, errors.New("closed Carrier acquisition is unavailable")
			}
		}
		if entry := pool.entries[key]; entry != nil && (entry.idleAt.IsZero() || pool.clock().UTC().Before(entry.idleAt.Add(closedCarrierRetention))) {
			if ctx.Err() != nil {
				pool.mu.Unlock()
				return nil, errors.New("closed Carrier acquisition is unavailable")
			}
			entry.active++
			entry.idleAt = time.Time{}
			pool.mu.Unlock()
			return &ClosedCarrierLease{pool: pool, key: key, entry: entry}, nil
		}
		var retired Carrier
		for previous, entry := range pool.entries {
			if previous.pair() == pair {
				delete(pool.entries, previous)
				retired = entry.carrier
			}
		}
		if retired == nil && len(pool.entries)+len(pool.operations) >= closedCarrierPoolMaximum {
			pool.mu.Unlock()
			return nil, errors.New("closed Carrier pool is exhausted")
		}
		operation := &closedCarrierOperation{done: make(chan struct{})}
		pool.operations[pair] = operation
		pool.mu.Unlock()
		return pool.openForPair(ctx, key, pair, operation, retired, validate, open)
	}
}

func (pool *ClosedCarrierPool) openForPair(ctx context.Context, key ClosedCarrierKey, pair closedCarrierPair, operation *closedCarrierOperation, retired Carrier, validate func() error, open func() (Carrier, error)) (*ClosedCarrierLease, error) {
	if retired != nil {
		if err := retired.Close(); err != nil {
			pool.finishOperation(pair, operation, err)
			return nil, err
		}
	}
	if ctx.Err() != nil || validate() != nil {
		pool.finishOperation(pair, operation, nil)
		return nil, errors.New("closed Carrier acquisition is unavailable")
	}
	pool.mu.Lock()
	closed := pool.closed
	pool.mu.Unlock()
	if closed {
		pool.finishOperation(pair, operation, nil)
		return nil, errors.New("closed Carrier pool is closed")
	}
	carrier, err := open()
	if err != nil || carrier == nil || ctx.Err() != nil || validate() != nil {
		var closeErr error
		if carrier != nil {
			closeErr = carrier.Close()
		}
		pool.finishOperation(pair, operation, closeErr)
		return nil, errors.New("closed Carrier acquisition is unavailable")
	}
	pool.mu.Lock()
	closed = pool.closed || ctx.Err() != nil
	if !closed {
		entry := &closedCarrierEntry{carrier: &closedCarrierRetirement{Carrier: carrier}, active: 1}
		pool.entries[key] = entry
		delete(pool.operations, pair)
		close(operation.done)
		pool.mu.Unlock()
		return &ClosedCarrierLease{pool: pool, key: key, entry: entry}, nil
	}
	pool.mu.Unlock()
	pool.finishOperation(pair, operation, carrier.Close())
	return nil, errors.New("closed Carrier pool is closed")
}

func (pool *ClosedCarrierPool) finishOperation(pair closedCarrierPair, operation *closedCarrierOperation, result error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.operations[pair] != operation {
		return
	}
	delete(pool.operations, pair)
	if result != nil {
		pool.closed = true
		pool.closeErr = errors.Join(pool.closeErr, result)
	}
	close(operation.done)
}

// MarkUsed records that this lease carried actual work. An opened but unused
// Carrier closes at Release instead of becoming speculative readiness.
func (lease *ClosedCarrierLease) MarkUsed() error {
	if lease == nil || lease.pool == nil {
		return errors.New("closed Carrier lease is unavailable")
	}
	lease.pool.mu.Lock()
	defer lease.pool.mu.Unlock()
	if lease.released || lease.pool.closed || lease.pool.entries[lease.key] != lease.entry {
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
		err := lease.entry.carrier.Close()
		lease.pool.closeErr = errors.Join(lease.pool.closeErr, err)
		return err
	}
	lease.entry.idleAt = lease.pool.clock().UTC()
	return nil
}

// Reap closes only idle, previously useful Carriers past the fixed retention.
func (pool *ClosedCarrierPool) Reap() error {
	if pool == nil {
		return nil
	}
	pool.mu.Lock()
	if pool.closed {
		result := pool.closeErr
		pool.mu.Unlock()
		return result
	}
	var retired []struct {
		pair      closedCarrierPair
		operation *closedCarrierOperation
		carrier   Carrier
	}
	for key, entry := range pool.entries {
		if entry.active == 0 && !entry.idleAt.IsZero() && !pool.clock().UTC().Before(entry.idleAt.Add(closedCarrierRetention)) {
			delete(pool.entries, key)
			operation := &closedCarrierOperation{done: make(chan struct{})}
			pool.operations[key.pair()] = operation
			retired = append(retired, struct {
				pair      closedCarrierPair
				operation *closedCarrierOperation
				carrier   Carrier
			}{key.pair(), operation, entry.carrier})
		}
	}
	pool.mu.Unlock()
	var result error
	for _, item := range retired {
		err := item.carrier.Close()
		pool.finishOperation(item.pair, item.operation, err)
		result = errors.Join(result, err)
	}
	return result
}

// Close releases every retained or active Carrier during Node withdrawal.
func (pool *ClosedCarrierPool) Close() error {
	if pool == nil {
		return nil
	}
	pool.mu.Lock()
	pool.closed = true
	operations := make([]*closedCarrierOperation, 0, len(pool.operations))
	for _, operation := range pool.operations {
		operations = append(operations, operation)
	}
	pool.mu.Unlock()
	for _, operation := range operations {
		<-operation.done
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()
	var result error
	for key, entry := range pool.entries {
		delete(pool.entries, key)
		result = errors.Join(result, entry.carrier.Close())
	}
	pool.closeErr = errors.Join(pool.closeErr, result)
	return pool.closeErr
}

func validClosedCarrierKey(key ClosedCarrierKey) bool {
	return key.NetworkID != [32]byte{} && key.ProfileDigest != [32]byte{} && key.LocalNodeID != [32]byte{} && key.PeerNodeID != [32]byte{} &&
		key.PeerKey != [32]byte{} && key.LocalNodeID != key.PeerNodeID && (key.CarrierProfile == ClosedCarrierTCP || key.CarrierProfile == ClosedCarrierQUIC)
}

func (key ClosedCarrierKey) pair() closedCarrierPair {
	return closedCarrierPair{local: key.LocalNodeID, peer: key.PeerNodeID}
}

// SameCarrier compares opaque Carrier incarnations, including after the first
// borrower's Release. It does not make either lease live or select a peer.
func (lease *ClosedCarrierLease) SameCarrier(other *ClosedCarrierLease) bool {
	return lease != nil && other != nil && lease.pool != nil && lease.pool == other.pool && lease.entry != nil && lease.entry == other.entry
}

// Invalidate closes only this exact incarnation. A late retained reader cannot
// remove a replacement acquired with the same public key after Reap or Release.
func (lease *ClosedCarrierLease) Invalidate() error {
	if lease == nil || lease.pool == nil {
		return nil
	}
	pool := lease.pool
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if pool.entries[lease.key] != lease.entry {
		return nil
	}
	delete(pool.entries, lease.key)
	err := lease.entry.carrier.Close()
	pool.closeErr = errors.Join(pool.closeErr, err)
	return err
}

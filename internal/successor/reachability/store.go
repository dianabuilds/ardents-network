package reachability

import (
	"errors"
	"path/filepath"
	"sync"
	"time"
)

const MaximumTargets = 128

// StoreConfig identifies one independent receiving history, not Network authority.
type StoreConfig struct {
	Root    string
	Network [32]byte
}

// Store owns one exclusive receiving root and signed conflict floors. The local
// holder history is a distinct owner; no Store root or lease is shared with it.
type Store struct {
	mu                sync.Mutex
	root              string
	network           [32]byte
	lease             storeLease
	floors            map[[32]byte]retainedFloor
	closed            bool
	failure, closeErr error
}

func OpenStore(config StoreConfig) (*Store, error) {
	if !storePlatformSupported || config.Root == "" || config.Network == [32]byte{} {
		return nil, errors.New("reachability Store configuration or platform unavailable")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err = prepareStoreRoot(root); err != nil {
		return nil, err
	}
	lease, err := acquireStoreLease(root)
	if err != nil {
		return nil, err
	}
	if err = initializeStoreRoot(root); err != nil {
		return nil, errors.Join(err, lease.release())
	}
	store := &Store{root: root, network: config.Network, lease: lease, floors: make(map[[32]byte]retainedFloor)}
	if err = store.restore(); err != nil {
		return nil, errors.Join(err, store.lease.release())
	}
	return store, nil
}

// Publish verifies delegation before mutation and persists conflict transitions
// before exposing them. check retains the receiving caller's original current
// duty/profile/time; it grants no proof authority and must not call into Store.
// A post-write refusal keeps the committed floor and permits no accepting ACK.
func (store *Store) Publish(raw []byte, profile [32]byte, at time.Time, check func() error) (Outcome, error) {
	if store == nil || check == nil {
		return Invalid, errors.New("reachability publication composition unavailable")
	}
	if err := check(); err != nil {
		return Invalid, err
	}
	proof, err := VerifyPublish(raw, store.network, profile, at)
	if err != nil {
		return Invalid, err
	}
	candidate := retainedFloor{descriptor: proof}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil {
		return Invalid, errors.Join(errors.New("reachability Store unavailable"), store.failure)
	}
	if err := check(); err != nil {
		return Invalid, err
	}
	outcome, next, decisionErr := Accepted, &candidate, error(nil)
	if prior, exists := store.floors[proof.Target]; exists {
		outcome, next, decisionErr = compareFloor(prior, candidate)
	} else if len(store.floors) >= MaximumTargets {
		return Invalid, errors.New("reachability Store Target capacity exhausted")
	}
	if next == nil {
		return outcome, errors.Join(decisionErr, check())
	}
	if err := store.write(*next); err != nil {
		store.failure = err
		return Invalid, errors.Join(decisionErr, err, check())
	}
	store.floors[proof.Target] = *next
	if err := check(); err != nil {
		return Invalid, errors.Join(decisionErr, err)
	}
	return outcome, decisionErr
}

// Lookup rechecks the highest retained proof at the caller's actual profile/time.
// Expiry or conflict never evicts its floor or reveals a predecessor.
func (store *Store) Lookup(target, profile [32]byte, at time.Time) ([]byte, Outcome, error) {
	if store == nil || target == [32]byte{} || profile == [32]byte{} || at.IsZero() {
		return nil, Invalid, errors.New("reachability lookup input invalid")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil {
		return nil, Invalid, errors.Join(errors.New("reachability Store unavailable"), store.failure)
	}
	floor, exists := store.floors[target]
	if !exists {
		return nil, Stale, nil
	}
	if floor.publicationConflict || floor.revisionConflict {
		return nil, Conflicting, nil
	}
	if _, err := Verify(floor.descriptor.raw, target, store.network, profile, at); err != nil {
		return nil, Stale, err
	}
	return floor.descriptor.Bytes(), Accepted, nil
}

// Close serializes through actual lease release and retains its first result.
func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return store.closeErr
	}
	store.closed = true
	store.closeErr = store.lease.release()
	clear(store.floors)
	return store.closeErr
}

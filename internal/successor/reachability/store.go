package reachability

import (
	"errors"
	"os"
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
	writer            sync.Mutex
	users             sync.WaitGroup
	closeOnce         sync.Once
	done              chan struct{}
	custody           *storeCustody
	root              string
	network           [32]byte
	lease             storeLease
	floors            map[[32]byte]retainedFloor
	closed            bool
	failure, closeErr error
	denial            error
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
	originalRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	lease, err := acquireStoreLease(originalRoot)
	if err != nil {
		return nil, errors.Join(err, originalRoot.Close())
	}
	lockInfo, err := originalRoot.Lstat(storeLockName)
	if err == nil {
		err = lease.check(lockInfo)
	}
	if err == nil {
		err = initializeStoreRoot(originalRoot)
	}
	if err != nil {
		return nil, errors.Join(err, originalRoot.Close(), lease.release())
	}
	store := &Store{root: root, network: config.Network, lease: lease, floors: make(map[[32]byte]retainedFloor), done: make(chan struct{})}
	store.custody, err = retainStoreCustody(originalRoot)
	if err != nil {
		return nil, errors.Join(err, store.lease.release())
	}
	if err = store.restore(); err != nil {
		return nil, errors.Join(err, store.custody.close(), store.lease.release())
	}
	if err = store.checkCustody(); err != nil {
		return nil, errors.Join(err, store.custody.close(), store.lease.release())
	}
	return store, nil
}

// Publish verifies delegation before mutation and persists conflict transitions
// before exposing them. check retains the receiving caller's original current
// duty/profile/time; it grants no proof authority. Check may inspect original
// custody without observer recursion; callbacks must not Publish or Close Store.
// A post-write refusal keeps the committed floor and permits no accepting ACK.
func (store *Store) Publish(raw []byte, profile [32]byte, at time.Time, check func() error) (Outcome, error) {
	if store == nil || check == nil {
		return Invalid, errors.New("reachability publication composition unavailable")
	}
	if err := store.borrow(); err != nil {
		return Invalid, err
	}
	defer store.users.Done()
	if err := check(); err != nil {
		return Invalid, err
	}
	proof, err := VerifyPublish(raw, store.network, profile, at)
	if err != nil {
		return Invalid, err
	}
	candidate := retainedFloor{descriptor: proof}
	// Serialize complete publication transitions independently of state. Foreign
	// observers and native I/O never run with mu held; Close denies immediately
	// and waits for this original borrow, including any writer waiter.
	store.writer.Lock()
	defer store.writer.Unlock()
	if err := store.checkCustody(); err != nil {
		return Invalid, err
	}
	if err := check(); err != nil {
		return Invalid, err
	}
	store.mu.Lock()
	if store.closed || store.failure != nil || store.denial != nil {
		failure := store.failure
		if failure == nil {
			failure = store.denial
		}
		store.mu.Unlock()
		if failure != nil {
			return Invalid, failure
		}
		return Invalid, errors.Join(errors.New("reachability Store unavailable"), failure)
	}
	outcome, next, decisionErr := Accepted, &candidate, error(nil)
	if prior, exists := store.floors[proof.Target]; exists {
		outcome, next, decisionErr = compareFloor(prior, candidate)
	} else if len(store.floors) >= MaximumTargets {
		store.mu.Unlock()
		return Invalid, errors.New("reachability Store Target capacity exhausted")
	}
	store.mu.Unlock()
	if next == nil {
		return outcome, errors.Join(decisionErr, check(), store.checkCustody(), store.localCheck())
	}
	if err := store.write(*next); err != nil {
		return Invalid, errors.Join(decisionErr, err, check())
	}
	if err := errors.Join(check(), store.checkCustody(), store.localCheck()); err != nil {
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
	if err := store.borrow(); err != nil {
		return nil, Invalid, err
	}
	defer store.users.Done()
	if err := store.checkCustody(); err != nil {
		return nil, Invalid, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil || store.denial != nil {
		if store.failure != nil {
			return nil, Invalid, store.failure
		}
		if store.denial != nil {
			return nil, Invalid, store.denial
		}
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

// Check reobserves this original Store's native custody. Request proof/refusal
// errors do not poison it; definite custody loss is permanently retained.
func (store *Store) Check() error {
	if err := store.borrow(); err != nil {
		return err
	}
	defer store.users.Done()
	if err := store.checkCustody(); err != nil {
		return err
	}
	return store.localCheck()
}

// OperationalFailure returns this original Store's first definite native
// failure, including one completed after caller denial or during joined Close.
// It performs no I/O, grants no borrow or proof, and excludes request refusal,
// caller denial and the separately retained physical Close result.
func (store *Store) OperationalFailure() error {
	if store == nil {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.failure
}

// Deny synchronously stops new borrows without waiting for native I/O or users.
// Original caller denial is separate from native owner failure and lease release.
// A later actual in-flight storage failure cannot be hidden by this notice.
func (store *Store) Deny(cause error) {
	if store == nil || cause == nil {
		return
	}
	store.mu.Lock()
	if store.denial == nil {
		store.denial = cause
	}
	store.mu.Unlock()
}

func (store *Store) borrow() error {
	if store == nil {
		return errors.New("reachability Store unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil || store.denial != nil {
		if store.failure != nil {
			return store.failure
		}
		if store.denial != nil {
			return store.denial
		}
		return errors.Join(errors.New("reachability Store unavailable"), store.failure)
	}
	store.users.Add(1)
	return nil
}

func (store *Store) localCheck() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil || store.denial != nil {
		if store.failure != nil {
			return store.failure
		}
		if store.denial != nil {
			return store.denial
		}
		return errors.Join(errors.New("reachability Store unavailable"), store.failure)
	}
	return nil
}

func (store *Store) fail(cause error) error {
	store.mu.Lock()
	if store.failure == nil {
		store.failure = cause
	}
	first := store.failure
	store.mu.Unlock()
	if first == cause {
		return cause
	}
	return errors.Join(first, cause)
}

// Close denies synchronously, joins original users, then releases native custody
// and lease once. Concurrent callers retain the same physical release result.
func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	store.mu.Lock()
	store.closed = true
	store.mu.Unlock()
	store.closeOnce.Do(func() {
		store.users.Wait()
		store.closeErr = errors.Join(store.custody.close(), store.lease.release())
		clear(store.floors)
		close(store.done)
	})
	<-store.done
	return store.closeErr
}

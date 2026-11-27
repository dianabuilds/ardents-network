package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
)

// Config identifies one independent receiving history, not Network authority.
type Config struct {
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
	floors            map[[32]byte]reachability.RetainedFloor
	closed            bool
	failure, closeErr error
	denial            error
}

func Open(config Config) (*Store, error) { return openStore(config, false) }

// OpenRetained requires complete existing history. It cannot initialize a
// missing root, marker, lock or records directory, nor repair an uncertain stage.
// Installation independently proves predecessor completion before this opening.
func OpenRetained(config Config) (*Store, error) { return openStore(config, true) }

func openStore(config Config, retained bool) (*Store, error) {
	if !storePlatformSupported || config.Root == "" || config.Network == [32]byte{} {
		return nil, errors.New("reachability Store configuration or platform unavailable")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if !retained {
		if err = prepareStoreRoot(root, syncStoreDirectory); err != nil {
			return nil, err
		}
	} else {
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.Join(errors.New("reachability retained root missing or invalid"), err)
		}
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
		if retained {
			err = checkRetainedStoreRoot(originalRoot)
		} else {
			err = initializeStoreRoot(originalRoot)
		}
	}
	if err != nil {
		return nil, errors.Join(err, originalRoot.Close(), lease.release())
	}
	store := &Store{root: root, network: config.Network, lease: lease, floors: make(map[[32]byte]reachability.RetainedFloor), done: make(chan struct{})}
	store.custody, err = retainStoreCustody(originalRoot)
	if err != nil {
		return nil, errors.Join(err, store.lease.release())
	}
	if err = store.restore(retained); err != nil {
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
func (store *Store) Publish(raw []byte, profile [32]byte, at time.Time, check func() error) (reachability.Outcome, error) {
	defer measurement.Begin(runtimeevent.ReachabilityPublish).End()

	if store == nil || check == nil {
		return reachability.Invalid, errors.New("reachability publication composition unavailable")
	}
	if err := store.borrow(); err != nil {
		return reachability.Invalid, err
	}
	defer store.users.Done()
	if err := check(); err != nil {
		return reachability.Invalid, err
	}
	proof, err := reachability.VerifyPublish(raw, store.network, profile, at)
	if err != nil {
		return reachability.Invalid, err
	}
	candidate := reachability.RetainedFloor{Proof: proof}
	// Serialize complete publication transitions independently of state. Foreign
	// observers and native I/O never run with mu held; Close denies immediately
	// and waits for this original borrow, including any writer waiter.
	store.writer.Lock()
	defer store.writer.Unlock()
	if err := store.checkCustody(); err != nil {
		return reachability.Invalid, err
	}
	if err := check(); err != nil {
		return reachability.Invalid, err
	}
	store.mu.Lock()
	if store.closed || store.failure != nil || store.denial != nil {
		failure := store.failure
		if failure == nil {
			failure = store.denial
		}
		store.mu.Unlock()
		if failure != nil {
			return reachability.Invalid, failure
		}
		return reachability.Invalid, errors.Join(errors.New("reachability Store unavailable"), failure)
	}
	outcome, next, decisionErr := reachability.Accepted, &candidate, error(nil)
	if prior, exists := store.floors[proof.Target]; exists {
		outcome, next, decisionErr = reachability.ResolveRetainedFloor(prior, candidate)
	} else if len(store.floors) >= reachability.MaximumTargets {
		store.mu.Unlock()
		return reachability.Invalid, errors.New("reachability Store Target capacity exhausted")
	}
	store.mu.Unlock()
	if next == nil {
		return outcome, errors.Join(decisionErr, check(), store.checkCustody(), store.localCheck())
	}
	if err := store.write(*next); err != nil {
		return reachability.Invalid, errors.Join(decisionErr, err, check())
	}
	if err := errors.Join(check(), store.checkCustody(), store.localCheck()); err != nil {
		return reachability.Invalid, errors.Join(decisionErr, err)
	}
	return outcome, decisionErr
}

// Lookup rechecks the highest retained proof at the caller's actual profile/time.
// Expiry or conflict never evicts its floor or reveals a predecessor.
func (store *Store) Lookup(target, profile [32]byte, at time.Time) ([]byte, reachability.Outcome, error) {
	defer measurement.Begin(runtimeevent.ReachabilityLookup).End()

	if store == nil || target == [32]byte{} || profile == [32]byte{} || at.IsZero() {
		return nil, reachability.Invalid, errors.New("reachability lookup input invalid")
	}
	if err := store.borrow(); err != nil {
		return nil, reachability.Invalid, err
	}
	defer store.users.Done()
	if err := store.checkCustody(); err != nil {
		return nil, reachability.Invalid, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failure != nil || store.denial != nil {
		if store.failure != nil {
			return nil, reachability.Invalid, store.failure
		}
		if store.denial != nil {
			return nil, reachability.Invalid, store.denial
		}
		return nil, reachability.Invalid, errors.Join(errors.New("reachability Store unavailable"), store.failure)
	}
	floor, exists := store.floors[target]
	if !exists {
		return nil, reachability.Stale, nil
	}
	if floor.PublicationConflict || floor.RevisionConflict {
		return nil, reachability.Conflicting, nil
	}
	raw := floor.Proof.Bytes()
	if _, err := reachability.Verify(raw, target, store.network, profile, at); err != nil {
		return nil, reachability.Stale, err
	}
	return raw, reachability.Accepted, nil
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

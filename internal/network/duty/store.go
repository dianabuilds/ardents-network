package duty

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

// Open claims one initialized local-role root, or creates it only when Create
// is explicit. Every retained generation is verified before return.
func Open(input Config) (*store, error) {
	return open(input, nil)
}
func open(input Config, ctx context.Context) (openedStore *store, resultErr error) {
	return openMode(input, ctx, false)
}

// Initialize creates only an empty first participation history in an existing
// vacant private directory. Existing leases, complete histories and partial
// attempts refuse without recovery or cleanup. It grants no participation.
func Initialize(ctx context.Context, input Config) error {
	if ctx == nil {
		return errors.New("local role initialization context is absent")
	}
	input.Create = true
	owner, err := openMode(input, ctx, true)
	if err != nil {
		return err
	}
	return errors.Join(owner.Close(), ctx.Err())
}

func openMode(input Config, ctx context.Context, fresh bool) (openedStore *store, resultErr error) {
	return openStoreMode(input, ctx, fresh, false)
}

// OpenRetainedOperation holds an existing participation lease without cleaning
// staging, recovering pointers or creating history. Recovery callers must refuse
// every uncertain prefix before deciding their one permitted domain effect.
func OpenRetainedOperation(ctx context.Context, input Config) (*store, error) {
	if ctx == nil || input.Create {
		return nil, errors.New("retained participation requires an existing history")
	}
	bounded, cancel := context.WithTimeout(ctx, operationLeaseWait)
	defer cancel()
	return openStoreMode(input, bounded, false, true)
}

func openStoreMode(input Config, ctx context.Context, fresh, retained bool) (openedStore *store, resultErr error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if input.Root == "" || input.Clock == nil || input.Clock().IsZero() {
		return nil, errors.New("local role configuration is incomplete")
	}
	root, err := filepath.Abs(input.Root)
	if err != nil {
		return nil, err
	}
	if fresh {
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.Join(errors.New("local role initialization requires an existing directory"), err)
		}
		if err := validateRootPermissions(root); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			return nil, errors.Join(errors.New("local role initialization requires a vacant root"), err)
		}
	}
	if err := inspectRoot(root, input.Create); err != nil {
		return nil, err
	}
	if retained {
		// Check types before any content open: a FIFO or device is never a
		// retained record, even before the exclusive lease can be acquired.
		if err := inspectRetainedEntries(root); err != nil {
			return nil, err
		}
	}
	if err := verifyRootCandidate(root, input.Create); err != nil {
		return nil, err
	}
	var lease rootLease
	if fresh {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lease, err = acquireFreshRootLease(root)
	} else if ctx == nil {
		lease, err = acquireRootLeaseMode(root, input.Create)
	} else {
		lease, err = acquireOperationLeaseMode(ctx, root, input.Create)
	}
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			if releaseErr := lease.release(); releaseErr != nil {
				resultErr = errors.Join(resultErr, releaseErr)
			}
		}
	}()
	if fresh {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
			return nil, errors.Join(errors.New("local role initialization root changed"), err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if err := verifyRootClaim(root, input.Create); err != nil {
		return nil, err
	}
	if retained {
		if err := validateRetainedRootPermissions(root); err != nil {
			return nil, err
		}
		if err := verifyRetainedHistory(root); err != nil {
			return nil, err
		}
	} else {
		if err := validateRootPermissions(root); err != nil {
			return nil, err
		}
		if err := prepareRoot(root, input.Create); err != nil {
			return nil, err
		}
	}
	state, current, err := loadState(root)
	if err != nil {
		return nil, err
	}
	store := &store{root: root, clock: input.Clock, lease: lease, state: state, current: current}
	if current == "" {
		if !input.Create {
			return nil, errors.New("local role state is not initialized")
		}
		if err := store.commit(durableState{Duties: []dutyRecord{}}); err != nil {
			return nil, err
		}
	}
	opened = true
	return store, nil
}

// Replace atomically replaces one producer's complete current duty set.
func (store *store) Replace(producer [32]byte, duties []Duty) error {
	return store.replace(producer, duties, false)
}

// Claim records a new producer only when no record for it is retained. Expiry
// does not establish the original producer's completion. The same exclusive
// operation observes absence and commits; this cannot adopt an old producer.
func (store *store) Claim(producer [32]byte, duties []Duty) error {
	if len(duties) == 0 {
		return errors.New("local role claim has no duties")
	}
	return store.replace(producer, duties, true)
}

func (store *store) replace(producer [32]byte, duties []Duty, claim bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || producer == ([32]byte{}) {
		if store.failed != nil {
			return errors.New("local role store requires restart after a failed commit")
		}
		return errors.New("local role replacement is invalid")
	}
	if claim {
		for _, record := range store.state.Duties {
			if record.Producer == producer {
				return ErrLocalRoleProducerRetained
			}
		}
	}
	records, err := proposeParticipation(store.state.Duties, producer, duties, store.clock().UTC())
	if err != nil {
		return err
	}
	next := durableState{Duties: records}
	return store.commit(next)
}

// Remove atomically removes every duty owned by producer.
func (store *store) Remove(producer [32]byte) error { return store.Replace(producer, nil) }

// Conflict reads the held current generation and reports one effective
// non-Initiator identity or family collision.
func (store *store) Conflict(identity, family [32]byte) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil {
		if store.failed != nil {
			return false, errors.New("local role store requires restart after a failed commit")
		}
		return false, errors.New("local role store is closed")
	}
	return participationConflict(store.state.Duties, identity, family, store.clock().UTC())
}

// Close releases the exclusive root lease once and retains its terminal result.
func (store *store) Close() error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return store.closeErr
	}
	store.closed = true
	store.closeErr = store.lease.release()
	return store.closeErr
}

// ErrInstallationSourceExhausted is returned when an installation cannot retain
// any additional effective `direct-source` Duty without exceeding the bounded
// exposure set. Callers may wrap it.
var ErrInstallationSourceExhausted = errors.New("direct-source exposure set is full")

// ErrLocalRoleRecordLimit reports that one local-role store would retain more
// duties than its fixed bound. It is distinct from the direct-source exposure
// bound, which returns ErrInstallationSourceExhausted.
var ErrLocalRoleRecordLimit = errors.New("local role record limit exceeded")

// ErrLocalRoleProducerLimit reports that one local-role store would retain
// duties for more producer identities than its fixed bound.
var ErrLocalRoleProducerLimit = errors.New("local role producer limit exceeded")

// ErrLocalRoleConflict reports an identity or family collision between
// retained non-Initiator duties.
var ErrLocalRoleConflict = errors.New("local role duty conflicts with retained state")

// ErrLocalRoleProducerRetained refuses a new lifetime's claim of a producer
// whose original records have not been removed after joined completion.
var ErrLocalRoleProducerRetained = errors.New("local role producer remains retained")

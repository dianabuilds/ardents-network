package duty

import (
	"context"
	"errors"
	"path/filepath"
)

// Open claims one initialized local-role root, or creates it only when Create
// is explicit. Every retained generation is verified before return.
func Open(input Config) (*store, error) {
	return open(input, nil)
}
func open(input Config, ctx context.Context) (openedStore *store, resultErr error) {
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
	if err := inspectRoot(root, input.Create); err != nil {
		return nil, err
	}
	if err := verifyRootCandidate(root, input.Create); err != nil {
		return nil, err
	}
	var lease rootLease
	if ctx == nil {
		lease, err = acquireRootLease(root)
	} else {
		lease, err = acquireOperationLease(ctx, root)
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
	if err := verifyRootClaim(root, input.Create); err != nil {
		return nil, err
	}
	if err := validateRootPermissions(root); err != nil {
		return nil, err
	}
	if err := prepareRoot(root, input.Create); err != nil {
		return nil, err
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
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || producer == ([32]byte{}) {
		if store.failed != nil {
			return errors.New("local role store requires restart after a failed commit")
		}
		return errors.New("local role replacement is invalid")
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

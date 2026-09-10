//go:build linux

package endpoint

import (
	"context"
	"errors"
)

// Serialize actual Source opening/issuance, never a publication ACK or a
// Service stream. Waiters own no tokens and remain cancellable by their caller
// and the independently authorized context. Validation runs after acquisition.
func (owner *textContext) acquireTextSourceOperation(ctx context.Context) (func(), error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Source operation unavailable")
	}
	owner.mu.Lock()
	if !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.mu.Unlock()
		return nil, errors.New("text Source context unavailable")
	}
	if owner.sourceOperations == nil {
		owner.sourceOperations = make(chan struct{}, 1)
	}
	operations, lease := owner.sourceOperations, owner.lease.Context()
	owner.mu.Unlock()
	select {
	case operations <- struct{}{}:
		if ctx.Err() != nil || lease.Err() != nil {
			<-operations
			return nil, errors.New("text Source operation cancelled")
		}
		return func() { <-operations }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lease.Done():
		return nil, lease.Err()
	}
}

// Reconcile the joined Source and reserve its next opening stock during actual
// caller work. No registration, Descriptor ACK or idle timer owns this gate.
func (owner *textContext) prepareTextSourceReady(ctx context.Context) error {
	release, err := owner.acquireTextSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	owner.mu.Lock()
	_, _, err = owner.textPermissionProfileLocked()
	missing := owner.prefix == nil
	owner.mu.Unlock()
	if err != nil {
		return err
	}
	if missing {
		if _, err := owner.openTextPrefix(ctx); err != nil {
			return err
		}
	}
	return owner.prepareTextSourceReopenOwned(ctx, nil)
}

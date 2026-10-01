//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
)

// Serialize actual Source opening/issuance, never a publication ACK or a
// Service stream. Waiters own no tokens and remain cancellable by their caller
// and the independently authorized context. Validation runs after acquisition.
func (owner *dutyContext) acquireSourceOperation(ctx context.Context) (func(), error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("text Source operation unavailable")
	}
	owner.mu.Lock()
	if !owner.liveLocked(owner.endpoint, owner.surface) {
		owner.mu.Unlock()
		return nil, errors.New("text Source context unavailable")
	}
	lifecycle, lease := &owner.source, owner.lease.Context()
	owner.mu.Unlock()
	return lifecycle.AcquireOperation(ctx, lease)
}

// Reconcile the joined Source and reserve its next opening stock during actual
// caller work. No registration, Descriptor ACK or idle timer owns this gate.
func (owner *dutyContext) prepareSourceReady(ctx context.Context) error {
	release, err := owner.acquireSourceOperation(ctx)
	if err != nil {
		return source.PreparationFailureAt("operation", err)
	}
	defer release()
	owner.mu.Lock()
	_, _, err = owner.permissionProfileLocked()
	missing := owner.source.CurrentLocked() == nil
	owner.mu.Unlock()
	if err != nil {
		return source.PreparationFailureAt("permission", err)
	}
	if missing {
		if _, err := owner.openPrefix(ctx); err != nil {
			return source.PreparationFailureAt("prefix-"+source.PrefixFailureStage(err), err)
		}
	}
	return owner.prepareSourceReopenOwned(ctx, nil)
}

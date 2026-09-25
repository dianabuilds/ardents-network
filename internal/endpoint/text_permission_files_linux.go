//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/permissionfile"
)

// The trusted Endpoint composition supplies these owner-only paths. Neither
// Application transport accepts a path, permission, holder key or context ID.
// Export returns the public commitment separately for explicit Custody approval.
func (owner *textContext) exportTextPermissionFile(ctx context.Context, path string, maxima [3]uint32) (commitment [32]byte, outcome error) {
	if ctx == nil || ctx.Err() != nil {
		return [32]byte{}, errors.New("text permission export canceled")
	}
	public, digest, err := owner.requestTextPermission(maxima)
	if err != nil {
		return [32]byte{}, err
	}
	defer clear(public)
	file, err := permissionfile.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer func() { outcome = errors.Join(outcome, file.Close()) }()
	if err := file.PublishRequest(public); err != nil {
		return [32]byte{}, err
	}
	// Exporting public bytes grants nothing. Recheck without generating another
	// holder if the hour changed while the file operation was in progress.
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	pending := owner.permission
	current := pending != nil && pending.digest == digest && pending.profile == profile &&
		!now.Before(pending.request.Permission.NotBefore) && now.Before(pending.request.Permission.NotAfter)
	owner.mu.Unlock()
	if err != nil || !current {
		return [32]byte{}, errors.New("text permission export context changed")
	}
	return digest, nil
}

// Import consumes the existing Custody permission format and the separately
// retained request digest. Files are transport, never authority or context
// restoration: the in-memory owner performs all currentness checks again.
func (owner *textContext) importTextPermissionFile(ctx context.Context, path string, digest [32]byte) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text permission import canceled")
	}
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	matches := owner.permission.matchesRequest(digest)
	owner.mu.Unlock()
	if err != nil || !matches {
		return errors.New("text permission import has no live request")
	}
	file, err := permissionfile.Open(path)
	if err != nil {
		return err
	}
	defer func() { outcome = errors.Join(outcome, file.Close()) }()
	raw, err := file.ReadResponse()
	if err != nil {
		return err
	}
	defer clear(raw)
	if err := ctx.Err(); err != nil {
		return err
	}
	return owner.importTextPermission(digest, raw)
}

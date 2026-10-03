//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
)

// Before the Source may become idle, obtain the two tokens that its next
// explicit read or scheduled publication will consume. This uses the current admitted Source,
// existing allocation and retained receivers, never another bootstrap allowance.
func (owner *dutyContext) prepareSourceReopen(ctx context.Context, flight *resolutionFlight) error {
	release, err := owner.acquireSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	return owner.prepareSourceReopenOwned(ctx, flight)
}

// The caller owns the Source operation; a nil flight belongs to readiness,
// which must not reserve or wait for a publication's resolution flight.
func (owner *dutyContext) prepareSourceReopenOwned(ctx context.Context, flight *resolutionFlight) error {
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.source.CurrentLocked() == nil || flight != nil && !owner.resolution.CurrentSourceLocked(flight, &owner.source) || !owner.tokens.PermissionLocked().Present() {
		owner.mu.Unlock()
		return source.PreparationFailureAt("stock", errors.New("text Source reopen stock unavailable"))
	}
	selection, err := owner.selectBootstrapLocked()
	if err != nil {
		owner.mu.Unlock()
		return source.PreparationFailureAt("selection", err)
	}
	missing := owner.tokens.PermissionLocked().MissingStockFor(profile.Digest, [][32]byte{selection.EntryNodeID, selection.InteriorNodeID}, 2)
	owner.mu.Unlock()
	if len(missing) != 0 {
		if err := owner.issueTokensForOpening(ctx, missing, 2, nil, false); err != nil {
			return source.PreparationFailureAt("issuance", err)
		}
	}
	return nil
}

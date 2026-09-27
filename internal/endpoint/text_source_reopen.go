//go:build linux

package endpoint

import (
	"context"
	"errors"
)

// Before the Source may become idle, obtain the two tokens that its next
// explicit read or scheduled publication will consume. This uses the current admitted Source,
// existing allocation and retained receivers, never another bootstrap allowance.
func (owner *textContext) prepareTextSourceReopen(ctx context.Context, flight *textResolutionFlight) error {
	release, err := owner.acquireTextSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	return owner.prepareTextSourceReopenOwned(ctx, flight)
}

// The caller owns the Source operation; a nil flight belongs to readiness,
// which must not reserve or wait for a publication's resolution flight.
func (owner *textContext) prepareTextSourceReopenOwned(ctx context.Context, flight *textResolutionFlight) error {
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.source.currentLocked() == nil || flight != nil && (owner.resolution != flight || flight.source == nil || !flight.source.currentLocked(&owner.source)) || owner.tokens.permission == nil {
		owner.mu.Unlock()
		return textSourcePreparationFailureAt("stock", errors.New("text Source reopen stock unavailable"))
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil {
		owner.mu.Unlock()
		return textSourcePreparationFailureAt("selection", err)
	}
	missing := owner.tokens.permission.missingStockFor(profile.Digest, [][32]byte{selection.EntryNodeID, selection.InteriorNodeID}, 2)
	owner.mu.Unlock()
	if len(missing) != 0 {
		if err := owner.issueTextTokensForOpening(ctx, missing, 2, nil, false); err != nil {
			return textSourcePreparationFailureAt("issuance", err)
		}
	}
	return nil
}

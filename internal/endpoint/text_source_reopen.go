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
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.resolution != flight || owner.prefix != flight.prefix || owner.permission == nil {
		owner.mu.Unlock()
		return errors.New("text Source reopen stock unavailable")
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil {
		owner.mu.Unlock()
		return err
	}
	var missing [][32]byte
	for _, receiver := range [][32]byte{selection.EntryNodeID, selection.InteriorNodeID} {
		ready := false
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == receiver && stock.challenge.ProfileDigest == profile.Digest &&
				stock.challenge.Class == 2 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) > 0 {
				ready = true
			}
		}
		if !ready {
			missing = append(missing, receiver)
		}
	}
	owner.mu.Unlock()
	if len(missing) != 0 {
		return owner.issueTextTokens(ctx, missing, 2)
	}
	return nil
}

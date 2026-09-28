//go:build linux

package endpoint

import (
	"context"
	"errors"
)

// prepareIssuerStock funds issuer admission only for current requested
// work. Before the first admitted prefix this uses the second bootstrap batch;
// thereafter the last Control token can replenish stock within its allocation.
func (owner *dutyContext) prepareIssuerStock(ctx context.Context, requested [][32]byte, class uint8,
	opening *operationFlight, acquisition joinAcquisition, expected *sourceHandle) error {
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.tokens.Permission == nil || !opening.admittedLocked(owner) ||
		!joinIssuanceCurrentLocked(owner, acquisition, expected) {
		owner.mu.Unlock()
		return errors.New("text issuer stock owner unavailable")
	}
	permission := owner.tokens.Permission
	if batch := permission.Pending; batch != nil {
		if !batch.Refill {
			owner.mu.Unlock()
			return nil // The requested batch keeps its original request/kind.
		}
		// Resume the exact internal stage before continuing the caller's work.
		// A failed refill cannot be replaced by a fresh batch or silently skipped.
		receivers := make([][32]byte, len(batch.Challenges))
		for index, challenge := range batch.Challenges {
			if challenge.Class != 1 || challenge.ReceiverNodeID != profile.IssuerNodeID {
				owner.mu.Unlock()
				return errors.New("text issuer pending stock binding unavailable")
			}
			receivers[index] = challenge.ReceiverNodeID
		}
		owner.mu.Unlock()
		return owner.issueTokensForOpeningWithCancellation(ctx, receivers, 1, opening, true, false, acquisition, expected)
	}
	self := class == 1 && len(requested) != 0
	for _, receiver := range requested {
		self = self && receiver == profile.IssuerNodeID
	}
	if self {
		owner.mu.Unlock()
		return nil
	}
	ready := permission.StockCountForDuty(profile.Digest, profile.IssuerNodeID, profile.IssuerDutyGeneration, 1)
	remaining := permission.Remaining(1)
	if ready >= 2 || remaining == 0 || owner.source.currentLocked() != nil && (ready == 0 || remaining < 2) {
		owner.mu.Unlock()
		return nil
	}
	count := min(remaining, 32)
	receivers := make([][32]byte, count)
	for index := range receivers {
		receivers[index] = profile.IssuerNodeID
	}
	owner.mu.Unlock()
	return owner.issueTokensForOpeningWithCancellation(ctx, receivers, 1, opening, true, false, acquisition, expected)
}

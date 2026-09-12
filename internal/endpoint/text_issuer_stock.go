//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// prepareTextIssuerStock funds issuer admission only for current requested
// work. Before the first admitted prefix this uses the second bootstrap batch;
// thereafter the last Control token can replenish stock within its allocation.
func (owner *textContext) prepareTextIssuerStock(ctx context.Context, requested [][32]byte, class uint8, opening *textSourceFlight) error {
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.permission == nil || owner.prefixOpening != opening || opening != nil && opening.context.Err() != nil {
		owner.mu.Unlock()
		return errors.New("text issuer stock owner unavailable")
	}
	permission := owner.permission
	if batch := permission.pending; batch != nil {
		if !batch.refill {
			owner.mu.Unlock()
			return nil // The requested batch keeps its original request/kind.
		}
		// Resume the exact internal stage before continuing the caller's work.
		// A failed refill cannot be replaced by a fresh batch or silently skipped.
		receivers := make([][32]byte, len(batch.challenges))
		for index, challenge := range batch.challenges {
			if challenge.Class != 1 || challenge.ReceiverNodeID != profile.IssuerNodeID {
				owner.mu.Unlock()
				return errors.New("text issuer pending stock binding unavailable")
			}
			receivers[index] = challenge.ReceiverNodeID
		}
		owner.mu.Unlock()
		return owner.issueTextTokensForOpening(ctx, receivers, 1, opening, true)
	}
	self := class == 1 && len(requested) != 0
	for _, receiver := range requested {
		self = self && receiver == profile.IssuerNodeID
	}
	if self {
		owner.mu.Unlock()
		return nil
	}
	ready := 0
	for _, stock := range permission.stock {
		if stock.challenge.ReceiverNodeID == profile.IssuerNodeID && stock.challenge.ReceiverDutyGeneration == profile.IssuerDutyGeneration &&
			stock.challenge.ProfileDigest == profile.Digest && stock.challenge.WindowStart == permission.accepted.NotBefore && stock.challenge.Class == 1 {
			ready += len(stock.tokens)
		}
	}
	remaining := permission.accepted.Maxima[0] - permission.reserved[0]
	if ready >= 2 || remaining == 0 || owner.prefix != nil && (ready == 0 || remaining < 2) {
		owner.mu.Unlock()
		return nil
	}
	count := min(remaining, 32)
	receivers := make([][32]byte, count)
	for index := range receivers {
		receivers[index] = profile.IssuerNodeID
	}
	owner.mu.Unlock()
	return owner.issueTextTokensForOpening(ctx, receivers, 1, opening, true)
}

func (owner *textContext) presentTextIssuerToken(flight *textIssuanceFlight, selection route.ClosedBootstrapSelection, hello route.ClosedHello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || flight == nil || owner.issuance != flight || flight.context == nil || flight.context.Err() != nil ||
		flight.prefix == nil || owner.prefix != flight.prefix || owner.permission == nil || owner.permission.pending == nil ||
		owner.permission.pending.prefix != flight.prefix || hello.Purpose != route.ClosedPurposeIssuer || class != 1 ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.RecipientNodeID != profile.IssuerNodeID || hello.RecipientDutyGeneration != profile.IssuerDutyGeneration ||
		hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text issuer token presentation authority unavailable")
	}
	current, err := owner.selectTextBootstrapLocked()
	if err != nil || current != selection {
		return nil, errors.New("text issuer token source changed")
	}
	return owner.takeTextTokenLocked(profile, now, hello, class, flight.context)
}

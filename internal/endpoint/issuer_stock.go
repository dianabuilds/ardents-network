//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// prepareIssuerStock funds issuer admission only for current requested
// work. Before the first admitted prefix this uses the second bootstrap batch;
// thereafter the last Control token can replenish stock within its allocation.
func (owner *dutyContext) prepareIssuerStock(ctx context.Context, requested [][32]byte, class uint8,
	opening *operationFlight, acquisition joinAcquisition, expected *sourceHandle) error {
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	if err != nil || ctx.Err() != nil || owner.tokens.permission == nil || !opening.admittedLocked(owner) ||
		!joinIssuanceCurrentLocked(owner, acquisition, expected) {
		owner.mu.Unlock()
		return errors.New("text issuer stock owner unavailable")
	}
	permission := owner.tokens.permission
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
	ready := permission.stockCountForDuty(profile.Digest, profile.IssuerNodeID, profile.IssuerDutyGeneration, 1)
	remaining := permission.remaining(1)
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

func (operation *issuanceOperation) presentIssuerToken(selection client.ClosedBootstrapSelection, hello ardp.Hello, class uint8) ([]byte, error) {
	owner := operation.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil || owner.tokens.issuance != operation || operation.context == nil || operation.context.Err() != nil ||
		operation.prefix == nil || !operation.prefix.currentLocked(&owner.source) || !owner.tokens.permission.pendingFor(operation.prefix) ||
		hello.Purpose != ardp.PurposeIssuer || class != 1 ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.RecipientNodeID != profile.IssuerNodeID || hello.RecipientDutyGeneration != profile.IssuerDutyGeneration ||
		hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text issuer token presentation authority unavailable")
	}
	current, err := owner.selectBootstrapLocked()
	if err != nil || current != selection {
		return nil, errors.New("text issuer token source changed")
	}
	return owner.takeTokenLocked(profile, now, hello, class, operation.context)
}

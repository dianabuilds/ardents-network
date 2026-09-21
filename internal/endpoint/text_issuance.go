//go:build linux

package endpoint

import (
	"context"
	"errors"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// Blinding state and finalized stock never leave this context. The network
// attempt owns copied request bytes only, so revocation can erase secrets
// under owner.mu while cancellation interrupts and joins the transport tree.
type textTokenBatch struct {
	refill     bool // Retained internal stock work; never receiver admission authority.
	prefix     *textSourceHandle
	challenges []credential.ClosedTokenContext
	selection  route.ClosedBootstrapSelection
	pending    *credential.PendingClosedTokenBatch
}

type textTokenStock struct {
	challenge credential.ClosedTokenContext
	tokens    [][]byte
}

// issueTextTokens is the trusted context owner's issuance operation. The
// retained Route members and intended receiver originate in Endpoint, never
// on a worker attachment. There is at most one live exchange per context.
func (owner *textContext) issueTextTokens(ctx context.Context, receivers [][32]byte, class uint8) error {
	return owner.issueTextTokensWithCancellation(ctx, receivers, class, false)
}

// A canceled recovery proposal cannot be retried by its completed logical
// stream. Burn its already-reserved allocation and erase only the batch that
// this proposal created, so it cannot block a later independent Service job.
func (owner *textContext) issueTextRecoveryTokens(ctx context.Context, receivers [][32]byte, class uint8) error {
	return owner.issueTextTokensWithCancellation(ctx, receivers, class, true)
}

func (owner *textContext) issueTextTokensWithCancellation(ctx context.Context, receivers [][32]byte, class uint8,
	discardCanceled bool) error {
	release, err := owner.acquireTextSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	return owner.issueTextTokensForOpeningWithCancellation(ctx, receivers, class, nil, false, discardCanceled)
}

// A non-nil opening must be the exact retained prefix transition. Keeping it
// across both bootstrap flights prevents unrelated issuance stealing its slot.
func (owner *textContext) issueTextTokensForOpening(ctx context.Context, receivers [][32]byte, class uint8, opening *textPrefixOpeningOperation, refill bool) error {
	return owner.issueTextTokensForOpeningWithCancellation(ctx, receivers, class, opening, refill, false)
}

func (owner *textContext) issueTextTokensForOpeningWithCancellation(ctx context.Context, receivers [][32]byte, class uint8,
	opening *textPrefixOpeningOperation, refill bool, discardCanceled bool) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || class < 1 || class > 3 || len(receivers) == 0 || len(receivers) > 32 {
		return errors.New("text issuance context is unavailable")
	}
	owner.mu.Lock()
	if !opening.admittedLocked(owner) {
		owner.mu.Unlock()
		return errors.New("text issuance prefix reservation unavailable")
	}
	hasPrefix := owner.currentTextSourceLocked() != nil
	owner.mu.Unlock()
	if hasPrefix && !refill {
		if err := owner.prepareTextIssuerStock(ctx, receivers, class, opening); err != nil {
			return err
		}
	}
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil {
		owner.mu.Unlock()
		return err
	}
	permission := owner.permission
	source, ok := owner.endpoint.closedState.(route.ClosedBootstrapState)
	if !ok || permission == nil || permission.accepted == (credential.Permission{}) || permission.profile != profile ||
		owner.issuance != nil || !opening.admittedLocked(owner) {
		owner.mu.Unlock()
		return errors.New("text issuance owner is unavailable")
	}
	selection, err := owner.selectTextBootstrapLocked()
	if err != nil || selection.ProfileDigest != profile.Digest {
		owner.mu.Unlock()
		return errors.New("text issuance source selection unavailable")
	}
	view, err := source.CurrentClosedRoute()
	if err != nil || view.Profile != profile || int(view.NodeCount) > len(view.Nodes) {
		owner.mu.Unlock()
		return errors.New("text issuance recipients are unavailable")
	}
	challenges := make([]credential.ClosedTokenContext, len(receivers))
	for index, receiver := range receivers {
		challenge := credential.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
			ReceiverNodeID: receiver, Class: class, WindowStart: permission.accepted.NotBefore}
		for _, node := range view.Nodes[:view.NodeCount] {
			if node.NodeID == receiver {
				if challenge.ReceiverDutyGeneration != 0 {
					owner.mu.Unlock()
					return errors.New("text issuance receiver is ambiguous")
				}
				challenge.ReceiverDutyGeneration = node.DutyGeneration
			}
		}
		if challenge.ReceiverDutyGeneration == 0 {
			owner.mu.Unlock()
			return errors.New("text issuance receiver is absent from State")
		}
		challenges[index] = challenge
	}
	batch := permission.pending
	if batch != nil {
		if batch.refill != refill || !slices.Equal(batch.challenges, challenges) || batch.selection != selection ||
			batch.prefix != nil && !batch.prefix.currentLocked(owner) {
			owner.mu.Unlock()
			return errors.New("text issuance retry must retain the original batch")
		}
	} else {
		if owner.currentTextSourceLocked() == nil && permission.batches >= 2 {
			owner.mu.Unlock()
			return errors.New("text bootstrap batch allowance is exhausted")
		}
		if uint32(len(challenges)) > permission.accepted.Maxima[class-1]-permission.reserved[class-1] {
			owner.mu.Unlock()
			return errors.New("text issuance allocation is exhausted")
		}
		pending, err := credential.PrepareClosedTokenBatch(credential.ClosedTokenBatchConfig{Profile: profile, Contexts: challenges,
			Permission: permission.accepted, HolderKey: permission.holder, Now: now})
		if err != nil {
			owner.mu.Unlock()
			return err
		}
		batch = &textTokenBatch{refill: refill, prefix: owner.currentTextSourceLocked(), challenges: challenges, selection: selection, pending: pending}
		permission.pending = batch
		permission.reserved[class-1] += uint32(len(challenges))
		if batch.prefix == nil {
			permission.batches++
		}
	}
	operation := newTextIssuanceOperation(owner, permission, profile, batch, discardCanceled)
	owner.issuance = operation
	owner.mu.Unlock()
	return operation.run(ctx, source, selection)
}

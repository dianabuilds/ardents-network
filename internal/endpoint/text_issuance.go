//go:build linux

package endpoint

import (
	"context"
	"errors"

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
	return owner.issueTextTokensForOpeningWithCancellation(ctx, receivers, class, nil, false, discardCanceled, nil, nil)
}

// issueTextJoinTokens retains issuance authority on the exact Source acquired
// by the JOIN. A replacement may cancel this work but cannot become its issuer.
func (owner *textContext) issueTextJoinTokens(ctx context.Context, receivers [][32]byte, class uint8,
	acquisition textJoinAcquisition, discardCanceled bool) error {
	release, err := owner.acquireTextSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	owner.mu.Lock()
	expected, current := acquisition.issuancePrefixLocked(owner)
	owner.mu.Unlock()
	if !current {
		return errors.New("text JOIN issuance Source acquisition unavailable")
	}
	err = owner.issueTextTokensForOpeningWithCancellation(ctx, receivers, class, nil, false, discardCanceled, acquisition, expected)
	owner.mu.Lock()
	_, current = acquisition.issuancePrefixLocked(owner)
	current = current && owner.currentTextSourceLocked() == expected
	owner.mu.Unlock()
	if err == nil && !current {
		return errors.New("text JOIN issuance Source acquisition changed")
	}
	return err
}

// A non-nil opening must be the exact retained prefix transition. Keeping it
// across both bootstrap flights prevents unrelated issuance stealing its slot.
func (owner *textContext) issueTextTokensForOpening(ctx context.Context, receivers [][32]byte, class uint8, opening *textPrefixOpeningOperation, refill bool) error {
	return owner.issueTextTokensForOpeningWithCancellation(ctx, receivers, class, opening, refill, false, nil, nil)
}

func (owner *textContext) issueTextTokensForOpeningWithCancellation(ctx context.Context, receivers [][32]byte, class uint8,
	opening *textPrefixOpeningOperation, refill bool, discardCanceled bool, acquisition textJoinAcquisition,
	expected *textSourceHandle) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || class < 1 || class > 3 || len(receivers) == 0 || len(receivers) > 32 {
		return errors.New("text issuance context is unavailable")
	}
	owner.mu.Lock()
	if !opening.admittedLocked(owner) || !textJoinIssuanceCurrentLocked(owner, acquisition, expected) {
		owner.mu.Unlock()
		return errors.New("text issuance prefix reservation unavailable")
	}
	hasPrefix := owner.currentTextSourceLocked() != nil
	owner.mu.Unlock()
	if hasPrefix && !refill {
		if err := owner.prepareTextIssuerStock(ctx, receivers, class, opening, acquisition, expected); err != nil {
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
		owner.issuance != nil || !opening.admittedLocked(owner) || !textJoinIssuanceCurrentLocked(owner, acquisition, expected) {
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
	batch, err := permission.reserveBatchLocked(profile, now, challenges, selection, refill, owner.currentTextSourceLocked(), acquisition != nil, expected)
	if err != nil {
		owner.mu.Unlock()
		return err
	}
	operation := newTextIssuanceOperation(owner, permission, profile, batch, discardCanceled)
	owner.issuance = operation
	owner.mu.Unlock()
	return operation.run(ctx, source, selection)
}

func textJoinIssuanceCurrentLocked(owner *textContext, acquisition textJoinAcquisition, expected *textSourceHandle) bool {
	if acquisition == nil {
		return true
	}
	prefix, current := acquisition.issuancePrefixLocked(owner)
	return current && prefix == expected && owner.currentTextSourceLocked() == expected
}

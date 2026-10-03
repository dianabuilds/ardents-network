//go:build linux

package endpoint

import (
	"context"
	"errors"
	admissiontoken "github.com/dianabuilds/ardents-network/internal/admission/token"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// issueTokens is the trusted context owner's issuance operation. The
// retained Route members and intended receiver originate in Endpoint, never
// on a worker attachment. There is at most one live exchange per context.
func (owner *dutyContext) issueTokens(ctx context.Context, receivers [][32]byte, class uint8) error {
	return owner.issueTokensWithCancellation(ctx, receivers, class, false)
}

// A canceled recovery proposal cannot be retried by its completed logical
// stream. Burn its already-reserved allocation and erase only the batch that
// this proposal created, so it cannot block a later independent Service job.
func (owner *dutyContext) issueRecoveryTokens(ctx context.Context, receivers [][32]byte, class uint8) error {
	return owner.issueTokensWithCancellation(ctx, receivers, class, true)
}

func (owner *dutyContext) issueTokensWithCancellation(ctx context.Context, receivers [][32]byte, class uint8,
	discardCanceled bool) error {
	release, err := owner.acquireSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	return owner.issueTokensForOpeningWithCancellation(ctx, receivers, class, nil, false, discardCanceled, nil, nil)
}

// issueJoinTokens retains issuance authority on the exact Source acquired
// by the JOIN. A replacement may cancel this work but cannot become its issuer.
func (owner *dutyContext) issueJoinTokens(ctx context.Context, receivers [][32]byte, class uint8,
	acquisition joinAcquisition, discardCanceled bool) error {
	release, err := owner.acquireSourceOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	owner.mu.Lock()
	expected, current := acquisition.IssuancePrefixLocked(owner)
	owner.mu.Unlock()
	if !current {
		return errors.New("text JOIN issuance Source acquisition unavailable")
	}
	err = owner.issueTokensForOpeningWithCancellation(ctx, receivers, class, nil, false, discardCanceled, acquisition, expected)
	owner.mu.Lock()
	_, current = acquisition.IssuancePrefixLocked(owner)
	current = current && owner.source.CurrentLocked() == expected
	owner.mu.Unlock()
	if err == nil && !current {
		return errors.New("text JOIN issuance Source acquisition changed")
	}
	return err
}

// A non-nil opening must be the exact retained prefix transition. Keeping it
// across both bootstrap flights prevents unrelated issuance stealing its slot.
func (owner *dutyContext) issueTokensForOpening(ctx context.Context, receivers [][32]byte, class uint8, opening *operationFlight, refill bool) error {
	return owner.issueTokensForOpeningWithCancellation(ctx, receivers, class, opening, refill, false, nil, nil)
}

func (owner *dutyContext) issueTokensForOpeningWithCancellation(ctx context.Context, receivers [][32]byte, class uint8,
	opening *operationFlight, refill bool, discardCanceled bool, acquisition joinAcquisition,
	expected *source.Handle) error {
	if owner == nil || ctx == nil || ctx.Err() != nil || class < 1 || class > 3 || len(receivers) == 0 || len(receivers) > 32 {
		return errors.New("text issuance context is unavailable")
	}
	owner.mu.Lock()
	if !opening.admittedLocked(owner) || !joinIssuanceCurrentLocked(owner, acquisition, expected) {
		owner.mu.Unlock()
		return errors.New("text issuance prefix reservation unavailable")
	}
	hasPrefix := owner.source.CurrentLocked() != nil
	owner.mu.Unlock()
	if hasPrefix && !refill {
		if err := owner.prepareIssuerStock(ctx, receivers, class, opening, acquisition, expected); err != nil {
			return err
		}
	}
	owner.mu.Lock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil {
		owner.mu.Unlock()
		return err
	}
	permission := owner.tokens.PermissionLocked()
	bootstrapState, ok := owner.endpoint.closedState.(client.ClosedBootstrapState)
	if !ok || !permission.CurrentFor(profile, now) ||
		owner.tokens.BusyLocked() || !opening.admittedLocked(owner) || !joinIssuanceCurrentLocked(owner, acquisition, expected) {
		owner.mu.Unlock()
		return errors.New("text issuance owner is unavailable")
	}
	selection, err := owner.selectBootstrapLocked()
	if err != nil || selection.ProfileDigest != profile.Digest {
		owner.mu.Unlock()
		return errors.New("text issuance source selection unavailable")
	}
	view, err := bootstrapState.CurrentClosedRoute()
	if err != nil || view.Profile != profile || int(view.NodeCount) > len(view.Nodes) {
		owner.mu.Unlock()
		return errors.New("text issuance recipients are unavailable")
	}
	challenges := make([]admissiontoken.ClosedTokenContext, len(receivers))
	for index, receiver := range receivers {
		challenge := admissiontoken.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID,
			ReceiverNodeID: receiver, Class: class, WindowStart: permission.Grant().NotBefore}
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
	operation, err := owner.tokens.BeginIssuanceLocked(stock.IssuanceIntent{
		Challenges: challenges, Selection: selection, Refill: refill, Prefix: prefixRef(owner.source.CurrentLocked()),
		Joined: acquisition != nil, ExpectedPrefix: prefixRef(expected), DiscardCanceled: discardCanceled,
	})
	if err != nil {
		owner.mu.Unlock()
		return err
	}
	owner.mu.Unlock()
	return operation.Run(ctx, bootstrapState, selection)
}

func joinIssuanceCurrentLocked(owner *dutyContext, acquisition joinAcquisition, expected *source.Handle) bool {
	if acquisition == nil {
		return true
	}
	prefix, current := acquisition.IssuancePrefixLocked(owner)
	return current && prefix == expected && owner.source.CurrentLocked() == expected
}

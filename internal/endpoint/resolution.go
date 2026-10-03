//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// lookupDescriptor consumes the context's actual issuer stock and journal,
// then verifies the full proof for the independently selected Target. It does
// not establish a Connection or accept a Descriptor as Introduction readiness.
func (owner *dutyContext) lookupDescriptor(ctx context.Context, target [32]byte) (verified reachability.Verified, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || target == [32]byte{} {
		return reachability.Verified{}, errors.New("text resolution input unavailable")
	}
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	if err != nil || owner.surface != broker.Connection || !owner.tokens.PermissionLocked().Present() || owner.source.CurrentLocked() == nil || owner.resolution.BusyLocked() || owner.source.OpeningInProgressLocked() || owner.tokens.BusyLocked() {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text resolution owner unavailable")
	}
	if !owner.descriptorHistory.CanAdmit(target) {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text Descriptor context capacity exhausted")
	}
	acquisition := owner.source.AcquireResolutionLocked()
	if acquisition == nil {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text resolution Source unavailable")
	}
	flight := owner.resolution.BeginLocked(owner.lease.Context(), ctx, acquisition)
	if flight == nil {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text resolution owner unavailable")
	}
	attempt := flight.context
	owner.mu.Unlock()
	defer func() { owner.finishResolution(flight, outcome, nil) }()
	receiver, err := flight.source.ResolutionRecipient()
	if err != nil {
		return reachability.Verified{}, err
	}
	flight.receiver = receiver // Fixed before the synchronous presenter is reachable.
	if err := owner.prepareSourceReopen(attempt, flight); err != nil {
		return reachability.Verified{}, err
	}
	if err := owner.ensureResolutionStock(flight); err != nil {
		return reachability.Verified{}, err
	}
	status, raw, err := flight.source.ExchangeDescriptor(attempt, func(hello ardp.Hello, class uint8) ([]byte, error) {
		return owner.presentResolutionToken(flight, hello, class)
	}, target, nil)
	defer clear(raw)
	if err != nil || status != 0 {
		return reachability.Verified{}, errors.Join(errors.New("text private resolution unavailable"), err)
	}
	return owner.acceptResolutionResult(ctx, flight, profile, target, raw)
}

func (owner *dutyContext) acceptResolutionResult(caller context.Context, flight *resolutionFlight,
	profile state.ClosedProfileView, target [32]byte, raw []byte) (reachability.Verified, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	current, now, err := owner.permissionProfileLocked()
	if caller == nil || err != nil || flight == nil || !owner.resolution.CurrentSourceLocked(flight, &owner.source) ||
		current != profile || flight.context.Err() != nil || caller.Err() != nil {
		return reachability.Verified{}, errors.New("text resolution authority changed")
	}
	return owner.descriptorHistory.Accept(raw, target, profile.NetworkID, profile.Digest, now)
}

func (owner *dutyContext) presentResolutionToken(flight *resolutionFlight, hello ardp.Hello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil || flight == nil || !owner.resolution.CurrentSourceLocked(flight, &owner.source) || flight.context.Err() != nil ||
		!owner.tokens.PermissionLocked().Present() || hello.Purpose != ardp.PurposeReachability || class != 1 || hello.RecipientNodeID != flight.receiver ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text resolution token authority unavailable")
	}
	receiver, err := flight.source.ResolutionRecipient()
	if err != nil || receiver != flight.receiver {
		return nil, errors.New("text resolution recipient changed")
	}
	return owner.tokens.TakeTokenLocked(profile, now, hello, class, flight.context)
}

// Reuse only unspent stock for this exact current recipient/window. A retained
// unfinished issuance batch must complete before another operation can use it.
func (owner *dutyContext) ensureResolutionStock(flight *resolutionFlight) error {
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	if err != nil || !owner.resolution.CurrentSourceLocked(flight, &owner.source) || flight.context.Err() != nil || !owner.tokens.PermissionLocked().Present() {
		owner.mu.Unlock()
		return errors.New("text resolution stock owner changed")
	}
	if !owner.tokens.PermissionLocked().HasPending() && owner.tokens.PermissionLocked().StockCountFor(profile.Digest, flight.receiver, 1) != 0 {
		owner.mu.Unlock()
		return nil
	}
	owner.mu.Unlock()
	return owner.issueTokens(flight.context, [][32]byte{flight.receiver}, 1)
}

func (owner *dutyContext) finishResolution(flight *resolutionFlight, outcome error, caller context.Context) error {
	owner.resolution.JoinCaller(flight)
	if caller != nil {
		outcome = errors.Join(outcome, caller.Err())
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.resolution.FinishLocked(flight, func() {
		if errors.Is(outcome, client.ErrClosedSourceCleanup) {
			owner.closeErr = errors.Join(owner.closeErr, outcome)
			owner.closed = true
			owner.endpoint.failDutyContexts(outcome)
		}
	})
	return outcome
}

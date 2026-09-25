//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

type textResolutionFlight struct {
	context       context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	source        textResolutionSource
	releaseSource func()
	receiver      [32]byte
}

type textResolutionSource interface {
	currentLocked(*textContext) bool
	resolutionRecipient() ([32]byte, error)
	exchangeDescriptor(context.Context, route.ClosedTokenPresenter, [32]byte, []byte) (uint8, []byte, error)
}

// lookupTextDescriptor consumes the context's actual issuer stock and journal,
// then verifies the full proof for the independently selected Target. It does
// not establish a Connection or accept a Descriptor as Introduction readiness.
func (owner *textContext) lookupTextDescriptor(ctx context.Context, target [32]byte) (verified reachability.Verified, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || target == [32]byte{} {
		return reachability.Verified{}, errors.New("text resolution input unavailable")
	}
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.surface != broker.Connection || owner.permission == nil || owner.currentTextSourceLocked() == nil || owner.resolution != nil || owner.source.openingInProgressLocked() || owner.issuance != nil {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text resolution owner unavailable")
	}
	if !owner.descriptorHistory.CanAdmit(target) {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text Descriptor context capacity exhausted")
	}
	source := owner.source.acquireResolutionLocked()
	if source == nil {
		owner.mu.Unlock()
		return reachability.Verified{}, errors.New("text resolution Source unavailable")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textResolutionFlight{context: attempt, cancel: cancel, done: make(chan struct{}), source: source, releaseSource: source.release}
	owner.resolution = flight
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	defer func() {
		cancel()
		if !stop() {
			<-interrupted
		}
		owner.finishTextResolution(flight, outcome)
	}()
	receiver, err := flight.source.resolutionRecipient()
	if err != nil {
		return reachability.Verified{}, err
	}
	flight.receiver = receiver // Fixed before the synchronous presenter is reachable.
	if err := owner.prepareTextSourceReopen(attempt, flight); err != nil {
		return reachability.Verified{}, err
	}
	if err := owner.ensureTextResolutionStock(flight); err != nil {
		return reachability.Verified{}, err
	}
	status, raw, err := flight.source.exchangeDescriptor(attempt, func(hello ardp.Hello, class uint8) ([]byte, error) {
		return owner.presentTextResolutionToken(flight, hello, class)
	}, target, nil)
	defer clear(raw)
	if err != nil || status != 0 {
		return reachability.Verified{}, errors.Join(errors.New("text private resolution unavailable"), err)
	}
	return owner.acceptTextResolutionResult(ctx, flight, profile, target, raw)
}

func (owner *textContext) acceptTextResolutionResult(caller context.Context, flight *textResolutionFlight,
	profile state.ClosedProfileView, target [32]byte, raw []byte) (reachability.Verified, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	current, now, err := owner.textPermissionProfileLocked()
	if caller == nil || err != nil || flight == nil || owner.resolution != flight || flight.source == nil ||
		current != profile || !flight.source.currentLocked(owner) || flight.context.Err() != nil || caller.Err() != nil {
		return reachability.Verified{}, errors.New("text resolution authority changed")
	}
	return owner.descriptorHistory.Accept(raw, target, profile.NetworkID, profile.Digest, now)
}

func (owner *textContext) presentTextResolutionToken(flight *textResolutionFlight, hello ardp.Hello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || flight == nil || owner.resolution != flight || flight.source == nil || !flight.source.currentLocked(owner) || flight.context.Err() != nil ||
		owner.permission == nil || hello.Purpose != ardp.PurposeReachability || class != 1 || hello.RecipientNodeID != flight.receiver ||
		hello.NetworkID != profile.NetworkID || hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest ||
		hello.ProfileDigest != profile.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text resolution token authority unavailable")
	}
	receiver, err := flight.source.resolutionRecipient()
	if err != nil || receiver != flight.receiver {
		return nil, errors.New("text resolution recipient changed")
	}
	return owner.takeTextTokenLocked(profile, now, hello, class, flight.context)
}

// Reuse only unspent stock for this exact current recipient/window. A retained
// unfinished issuance batch must complete before another operation can use it.
func (owner *textContext) ensureTextResolutionStock(flight *textResolutionFlight) error {
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.resolution != flight || flight.source == nil || !flight.source.currentLocked(owner) || flight.context.Err() != nil || owner.permission == nil {
		owner.mu.Unlock()
		return errors.New("text resolution stock owner changed")
	}
	if !owner.permission.hasPending() && owner.permission.stockCountFor(profile.Digest, flight.receiver, 1) != 0 {
		owner.mu.Unlock()
		return nil
	}
	owner.mu.Unlock()
	return owner.issueTextTokens(flight.context, [][32]byte{flight.receiver}, 1)
}

func (owner *textContext) finishTextResolution(flight *textResolutionFlight, outcome error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if flight.releaseSource != nil {
		flight.releaseSource()
		flight.releaseSource = nil
	}
	if errors.Is(outcome, route.ErrClosedSourceCleanup) {
		owner.closeErr = errors.Join(owner.closeErr, outcome)
		owner.closed = true
		owner.endpoint.failTextContexts(outcome)
	}
	if owner.resolution == flight {
		owner.resolution = nil
	}
	close(flight.done)
}

//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

type publisherPrefixOpening interface {
	openingAvailableLocked() bool
	membersSlotLocked() **interiorSet
	reserveOpeningLocked(*operationFlight) bool
	openingCurrentLocked(*operationFlight) bool
	finishOpeningLocked(*operationFlight, *client.ClosedSourcePrefix, context.CancelFunc, bool) bool
}

func (owner *textContext) openResponderPrefix(ctx context.Context) (*responderPrefixHandle, error) {
	if owner == nil {
		return nil, errors.New("text Publisher owner unavailable")
	}
	opened, err := owner.openPublisherPrefix(ctx, &owner.responder, 3)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	handle := owner.responder.acquireOpenedLocked(opened)
	owner.mu.Unlock()
	if handle == nil {
		return nil, errors.New("text Responder prefix unavailable after opening")
	}
	return handle, nil
}

func (owner *textContext) openIntroductionPrefix(ctx context.Context) (*introductionPrefixHandle, error) {
	if owner == nil {
		return nil, errors.New("text Publisher owner unavailable")
	}
	opened, err := owner.openPublisherPrefix(ctx, &owner.introduction.prefix, 4)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	handle := owner.introduction.prefix.acquireOpenedLocked(opened)
	owner.mu.Unlock()
	if handle == nil {
		return nil, errors.New("text Introduction prefix unavailable after opening")
	}
	return handle, nil
}

// Both Publisher domains share admission/lifetime rules but retain distinct
// allocations, transports and stock. This private selector grants no authority.
func (owner *textContext) openPublisherPrefix(ctx context.Context, role publisherPrefixOpening, domain uint8) (*client.ClosedSourcePrefix, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || !(domain == 4 && role == &owner.introduction.prefix || domain == 3 && role == &owner.responder) {
		return nil, errors.New("text Publisher role context unavailable")
	}
	owner.mu.Lock()
	_, _, err := owner.permissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || owner.source.currentLocked() == nil || owner.tokens.permission == nil || owner.source.openingInProgressLocked() || !role.openingAvailableLocked() {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher role owner unavailable")
	}
	source, ok := owner.endpoint.closedState.(client.ClosedBootstrapState)
	if !ok {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher State unavailable")
	}
	selection, err := owner.selectAdjacentLocked(domain, role.membersSlotLocked())
	if err != nil {
		owner.mu.Unlock()
		return nil, err
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &operationFlight{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
	if !role.reserveOpeningLocked(flight) {
		owner.mu.Unlock()
		cancel()
		return nil, errors.New("text Publisher role owner unavailable")
	}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	var prefix *client.ClosedSourcePrefix
	openErr := owner.ensurePublisherStock(role, flight, selection)
	if openErr == nil {
		open := client.OpenClosedIntroductionPrefix
		if domain == 3 {
			open = client.OpenClosedResponderPrefix
		}
		prefix, openErr = open(attempt, source, selection, func(hello ardp.Hello, class uint8) ([]byte, error) {
			return owner.presentPublisherForwardingToken(role, domain, flight, selection, hello, class)
		})
	}
	if !stop() {
		<-interrupted
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	publish := openErr == nil && ctx.Err() == nil && attempt.Err() == nil && owner.liveLocked(owner.endpoint, broker.Administration)
	owned := role.finishOpeningLocked(flight, prefix, cancel, publish)
	if !owned || !publish {
		cancel()
		cleanup := prefix.Close()
		if errors.Is(openErr, client.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, openErr, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		return nil, errors.Join(openErr, ctx.Err(), cleanup, errors.New("text Publisher role prefix unavailable"))
	}
	return prefix, nil
}

func (owner *textContext) ensurePublisherStock(role publisherPrefixOpening, flight *operationFlight, selection client.ClosedBootstrapSelection) error {
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	if err != nil || !role.openingCurrentLocked(flight) || flight.context.Err() != nil || owner.tokens.permission == nil {
		owner.mu.Unlock()
		return errors.New("text Publisher role stock unavailable")
	}
	pending := owner.tokens.permission.hasPending()
	missing := owner.tokens.permission.missingStockFor(profile.Digest, [][32]byte{selection.EntryNodeID, selection.InteriorNodeID}, 2)
	owner.mu.Unlock()
	if len(missing) != 0 {
		// The issuance owner resumes only an exact retained batch (including
		// an internal issuer refill); it refuses any foreign request.
		return owner.issueTokens(flight.context, missing, 2)
	}
	if pending {
		return errors.New("text Publisher role cannot skip pending issuance")
	}
	return nil
}

func (owner *textContext) presentPublisherForwardingToken(role publisherPrefixOpening, domain uint8, flight *operationFlight, selection client.ClosedBootstrapSelection, hello ardp.Hello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || !role.openingCurrentLocked(flight) || flight.context.Err() != nil || owner.tokens.permission == nil ||
		class != 2 || hello.Purpose != ardp.PurposeForwarding || hello.NetworkID != profile.NetworkID || hello.ProfileDigest != profile.Digest ||
		hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text Publisher role forwarding authority unavailable")
	}
	current, err := owner.selectAdjacentLocked(domain, role.membersSlotLocked())
	if err != nil || current != selection || hello.RecipientNodeID != selection.EntryNodeID && hello.RecipientNodeID != selection.InteriorNodeID {
		return nil, errors.New("text Publisher role selection changed")
	}
	return owner.takeTokenLocked(profile, now, hello, class, flight.context)
}

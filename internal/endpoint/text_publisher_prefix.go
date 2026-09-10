//go:build linux

package endpoint

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Each Publisher role retains a distinct tree funded through the Source
// prefix. Neither a worker nor a receiver chooses its members or tokens.
type textPublisherPrefix struct {
	prefix  *route.ClosedSourcePrefix
	cancel  context.CancelFunc
	opening *textSourceFlight
	set     *textSourceSet
}

func (owner *textContext) openTextIntroductionPrefix(ctx context.Context) (*route.ClosedSourcePrefix, error) {
	if owner == nil {
		return nil, errors.New("text Publisher owner unavailable")
	}
	return owner.openTextPublisherPrefix(ctx, &owner.introduction, 4)
}

// Both Publisher domains share admission/lifetime rules but retain distinct
// allocations, transports and stock. This private selector grants no authority.
func (owner *textContext) openTextPublisherPrefix(ctx context.Context, role *textPublisherPrefix, domain uint8) (*route.ClosedSourcePrefix, error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || !(domain == 4 && role == &owner.introduction || domain == 3 && role == &owner.responder) {
		return nil, errors.New("text Publisher role context unavailable")
	}
	owner.mu.Lock()
	_, _, err := owner.textPermissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || owner.prefix == nil || owner.permission == nil || owner.issuance != nil || owner.prefixOpening != nil || role.opening != nil || role.prefix != nil {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher role owner unavailable")
	}
	source, ok := owner.endpoint.closedState.(route.ClosedBootstrapState)
	if !ok {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher State unavailable")
	}
	selection, err := owner.selectTextAdjacentLocked(domain, &role.set)
	if err != nil {
		owner.mu.Unlock()
		return nil, err
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textSourceFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	role.opening = flight
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	var prefix *route.ClosedSourcePrefix
	openErr := owner.ensureTextPublisherStock(role, flight, selection)
	if openErr == nil {
		open := route.OpenClosedIntroductionPrefix
		if domain == 3 {
			open = route.OpenClosedResponderPrefix
		}
		prefix, openErr = open(attempt, source, selection, func(hello route.ClosedHello, class uint8) ([]byte, error) {
			return owner.presentTextPublisherForwardingToken(role, domain, flight, selection, hello, class)
		})
	}
	if !stop() {
		<-interrupted
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	role.opening = nil
	if openErr != nil || ctx.Err() != nil || attempt.Err() != nil || !owner.liveLocked(owner.endpoint, broker.Administration) {
		cancel()
		cleanup := prefix.Close()
		if errors.Is(openErr, route.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, openErr, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		return nil, errors.Join(openErr, ctx.Err(), cleanup, errors.New("text Publisher role prefix unavailable"))
	}
	role.prefix, role.cancel = prefix, cancel
	return prefix, nil
}

func (owner *textContext) ensureTextPublisherStock(role *textPublisherPrefix, flight *textSourceFlight, selection route.ClosedBootstrapSelection) error {
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil || role.opening != flight || flight.context.Err() != nil || owner.permission == nil {
		owner.mu.Unlock()
		return errors.New("text Publisher role stock unavailable")
	}
	pending := owner.permission.pending != nil
	var missing [][32]byte
	for _, receiver := range [][32]byte{selection.EntryNodeID, selection.InteriorNodeID} {
		found := false
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == receiver && stock.challenge.ProfileDigest == profile.Digest && stock.challenge.Class == 2 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) != 0 {
				found = true
			}
		}
		if !found {
			missing = append(missing, receiver)
		}
	}
	owner.mu.Unlock()
	if len(missing) != 0 {
		// The issuance owner resumes only an exact retained batch (including
		// an internal issuer refill); it refuses any foreign request.
		return owner.issueTextTokens(flight.context, missing, 2)
	}
	if pending {
		return errors.New("text Publisher role cannot skip pending issuance")
	}
	return nil
}

func (owner *textContext) presentTextPublisherForwardingToken(role *textPublisherPrefix, domain uint8, flight *textSourceFlight, selection route.ClosedBootstrapSelection, hello route.ClosedHello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || role.opening != flight || flight.context.Err() != nil || owner.permission == nil ||
		class != 2 || hello.Purpose != route.ClosedPurposeForwarding || hello.NetworkID != profile.NetworkID || hello.ProfileDigest != profile.Digest ||
		hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text Publisher role forwarding authority unavailable")
	}
	current, err := owner.selectTextAdjacentLocked(domain, &role.set)
	if err != nil || current != selection || hello.RecipientNodeID != selection.EntryNodeID && hello.RecipientNodeID != selection.InteriorNodeID {
		return nil, errors.New("text Publisher role selection changed")
	}
	return owner.takeTextTokenLocked(profile, now, hello, class, flight.context)
}

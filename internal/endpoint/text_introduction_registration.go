//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

type textRegistrationFlight struct {
	previous *textIntroductionRegistration
	context  context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	prefix   *route.ClosedSourcePrefix
	receiver [32]byte
}

type textIntroductionRegistration struct {
	createdAt     time.Time
	refreshAt     time.Time
	published     bool
	publishedAt   time.Time // First verified ACK transition, retained across exact retries.
	recipient     *instance.PrivateRecipient
	recipientDone <-chan struct{}
	descriptor    []byte
	channel       *route.ClosedIntroductionRegistration
	node          [32]byte
	request       route.ClosedRegistrationRequest
	cancel        context.CancelFunc
}

// registerTextIntroduction owns the fresh random slot and spends a real
// Publication token on the separate admitted Introduction tree. Registration
// supplies no Service authority and is not Descriptor publication readiness.
func (owner *textContext) registerTextIntroduction(ctx context.Context, revision uint64, expiry time.Time) (*textIntroductionRegistration, error) {
	return owner.openTextRegistration(ctx, revision, expiry, nil)
}

func (owner *textContext) openTextRegistration(ctx context.Context, revision uint64, expiry time.Time, previous *textIntroductionRegistration) (registered *textIntroductionRegistration, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || revision == 0 {
		return nil, errors.New("text Publisher registration unavailable")
	}
	owner.mu.Lock()
	profile, now, err := owner.textPermissionProfileLocked()
	if previous == nil && owner.registration != nil && owner.withdrawal == nil {
		select {
		case <-owner.registration.channel.Done():
			cleanup := owner.registration.close()
			owner.registration.cancel()
			owner.registration = nil
			owner.signalTextRegistrationsLocked()
			if cleanup != nil {
				owner.closeErr = errors.Join(owner.closeErr, cleanup)
				owner.closed = true
				owner.endpoint.failTextContexts(cleanup)
				err = errors.Join(err, cleanup)
			}
		default:
		}
	}
	if err != nil || owner.surface != broker.Administration || owner.introduction.prefix == nil || owner.introduction.opening != nil || owner.registrationOpening != nil || owner.withdrawal != nil || owner.registration != previous || previous != nil && (owner.previousRegistration != nil || owner.refresh == nil || owner.refresh.context != ctx || previous.recipient == nil || revision <= previous.request.Revision) || owner.permission == nil || owner.issuance != nil || !now.Before(expiry) || expiry.After(now.Add(600*time.Second)) {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher registration owner unavailable")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textRegistrationFlight{previous: previous, context: attempt, cancel: cancel, done: make(chan struct{}), prefix: owner.introduction.prefix}
	owner.registrationOpening = flight
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	var channel *route.ClosedIntroductionRegistration
	defer func() {
		if !stop() {
			<-interrupted
		}
		registered, outcome = owner.finishTextRegistration(ctx, flight, registered, channel, outcome)
	}()
	receiver, until, err := flight.prefix.IntroductionRecipient()
	if err != nil || expiry.After(until) {
		return nil, errors.Join(err, errors.New("text Publisher registration expiry exceeds current duty"))
	}
	flight.receiver = receiver
	owner.mu.Lock()
	ready := false
	if owner.permission != nil && owner.permission.pending == nil {
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == receiver && stock.challenge.ProfileDigest == profile.Digest && stock.challenge.Class == 3 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) != 0 {
				ready = true
			}
		}
	}
	owner.mu.Unlock()
	if !ready {
		if err := owner.issueTextTokens(attempt, [][32]byte{receiver}, 3); err != nil {
			return nil, err
		}
	}
	request := route.ClosedRegistrationRequest{Revision: revision, Expiry: expiry}
	if _, err := rand.Read(request.Slot[:]); err != nil {
		return nil, err
	}
	createdAt := owner.endpoint.clock().UTC().Truncate(time.Second)
	channel, err = flight.prefix.RegisterIntroduction(attempt, func(hello route.ClosedHello, class uint8) ([]byte, error) {
		return owner.presentTextRegistrationToken(flight, hello, class)
	}, request)
	if err != nil {
		return nil, err
	}
	select {
	case <-channel.Done():
		return nil, errors.New("text Publisher registration ended before handover")
	default:
	}
	return &textIntroductionRegistration{createdAt: createdAt, channel: channel, node: receiver, request: request, cancel: cancel}, nil
}

func (owner *textContext) finishTextRegistration(ctx context.Context, flight *textRegistrationFlight, registered *textIntroductionRegistration, channel *route.ClosedIntroductionRegistration, outcome error) (*textIntroductionRegistration, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	owner.registrationOpening = nil
	if outcome != nil || ctx.Err() != nil || flight.context.Err() != nil || owner.registration != flight.previous || !owner.liveLocked(owner.endpoint, broker.Administration) {
		flight.cancel()
		cleanup := channel.Close()
		if errors.Is(outcome, route.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, outcome, cleanup)
			owner.closed = true
			owner.endpoint.failTextContexts(owner.closeErr)
		}
		return nil, errors.Join(outcome, ctx.Err(), cleanup, errors.New("text Publisher registration did not complete"))
	}
	if flight.previous != nil {
		// Registration alone cannot switch published readiness or shorten its
		// predecessor. The verified Descriptor ACK establishes the overlap.
		owner.previousRegistration, owner.previousUntil = flight.previous, flight.previous.request.Expiry
	}
	owner.registration = registered
	owner.signalTextRegistrationsLocked()
	return registered, nil
}

func (owner *textContext) presentTextRegistrationToken(flight *textRegistrationFlight, hello route.ClosedHello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.textPermissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || owner.registrationOpening != flight || owner.introduction.prefix != flight.prefix || flight.context.Err() != nil || owner.permission == nil ||
		class != 3 || hello.Purpose != route.ClosedPurposeIntroduction || hello.RecipientNodeID != flight.receiver || hello.NetworkID != profile.NetworkID || hello.ProfileDigest != profile.Digest ||
		hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text Publisher registration token authority unavailable")
	}
	receiver, _, err := flight.prefix.IntroductionRecipient()
	if err != nil || receiver != flight.receiver {
		return nil, errors.New("text Publisher registration recipient changed")
	}
	return owner.takeTextTokenLocked(profile, now, hello, class, flight.context)
}

func (owner *textContext) withdrawTextIntroduction(ctx context.Context) error {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text Publisher registration unavailable")
	}
	owner.mu.Lock()
	registered := owner.registration
	if registered == nil || owner.resolution != nil || owner.registrationOpening != nil || owner.withdrawal != nil || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return errors.New("text Publisher registration absent or ending")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textSourceFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	owner.withdrawal = flight
	if owner.refresh != nil {
		owner.refresh.cancel()
	}
	owner.mu.Unlock()
	owner.stopTextRefresh()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	err := registered.channel.Withdraw(attempt)
	cancel()
	if !stop() {
		<-interrupted
	}
	registered.cancel()
	cleanup := registered.close()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.registration == registered {
		owner.registration = nil
	}
	previous := owner.previousRegistration
	owner.previousRegistration = nil
	owner.signalTextRegistrationsLocked()
	if previous != nil {
		previous.cancel()
		cleanup = errors.Join(cleanup, previous.close())
	}
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failTextContexts(cleanup)
	}
	owner.withdrawal = nil
	close(flight.done)
	return errors.Join(err, cleanup)
}

func (registered *textIntroductionRegistration) close() error {
	if registered == nil {
		return nil
	}
	err := registered.channel.Close()
	if registered.recipientDone != nil {
		<-registered.recipientDone
	}
	return err
}

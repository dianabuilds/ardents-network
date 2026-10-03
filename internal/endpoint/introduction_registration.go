//go:build linux

package endpoint

import (
	"context"
	"crypto/rand"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// registrationFlight fuses one registration opening with the duty context
// lifetime. It stays in the root because its prefix belongs to the root
// Introduction prefix lifecycle, and satisfies the pair's FlightRef seam.
type registrationFlight struct {
	previous *introduction.Registration
	context  context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	prefix   *introductionPrefixHandle
	receiver [32]byte
}

func (flight *registrationFlight) CancelFlight() {
	if flight != nil {
		flight.cancel()
	}
}

func (flight *registrationFlight) JoinFlight() {
	if flight != nil {
		<-flight.done
	}
}

// registerIntroduction owns the fresh random slot and spends a real
// Publication token on the separate admitted Introduction tree. Registration
// supplies no Service authority and is not Descriptor publication readiness.
func (owner *dutyContext) registerIntroduction(ctx context.Context, revision uint64, expiry time.Time) (*introduction.Registration, error) {
	return owner.openRegistration(ctx, revision, expiry, nil)
}

func (owner *dutyContext) openRegistration(ctx context.Context, revision uint64, expiry time.Time, previous *introduction.Registration) (registered *introduction.Registration, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || revision == 0 {
		return nil, errors.New("text Publisher registration unavailable")
	}
	owner.mu.Lock()
	profile, now, err := owner.permissionProfileLocked()
	if previous == nil {
		if ended := owner.publication.pair.EvictEndedTargetLocked(); ended != nil {
			cleanup := ended.Close()
			ended.Cancel()
			owner.publication.signalRegistrationsLocked()
			if cleanup != nil {
				owner.closeErr = errors.Join(owner.closeErr, cleanup)
				owner.closed = true
				owner.endpoint.failDutyContexts(cleanup)
				err = errors.Join(err, cleanup)
			}
		}
	}
	prefix := owner.introduction.prefix.currentLocked()
	prior, _ := owner.publication.pair.PreviousLocked()
	if err != nil || owner.surface != broker.Administration || prefix == nil || owner.introduction.prefix.openingInProgressLocked() || previous != nil && (prior != nil || !owner.publication.refresh.MatchesContext(ctx) || !previous.HasRecipientLocked() || revision <= previous.Revision()) || !owner.tokens.PermissionLocked().Present() || !now.Before(expiry) || expiry.After(now.Add(600*time.Second)) {
		owner.mu.Unlock()
		return nil, errors.New("text Publisher registration owner unavailable")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &registrationFlight{previous: previous, context: attempt, cancel: cancel, done: make(chan struct{}), prefix: prefix}
	if !owner.publication.pair.BeginOpeningLocked(flight, previous) {
		owner.mu.Unlock()
		cancel()
		return nil, errors.New("text Publisher registration owner unavailable")
	}
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	var channel *client.ClosedIntroductionRegistration
	defer func() {
		if !stop() {
			<-interrupted
		}
		registered, outcome = owner.finishRegistration(ctx, flight, registered, channel, outcome)
	}()
	receiver, until, err := flight.prefix.introductionRecipient()
	if err != nil || expiry.After(until) {
		return nil, errors.Join(err, errors.New("text Publisher registration expiry exceeds current duty"))
	}
	flight.receiver = receiver
	owner.mu.Lock()
	ready := !owner.tokens.PermissionLocked().HasPending() &&
		owner.tokens.PermissionLocked().StockCountFor(profile.Digest, receiver, 3) != 0
	owner.mu.Unlock()
	if !ready {
		if err := owner.issueTokens(attempt, [][32]byte{receiver}, 3); err != nil {
			return nil, err
		}
	}
	request := terminal.RegistrationRequest{Revision: revision, Expiry: expiry}
	if _, err := rand.Read(request.Slot[:]); err != nil {
		return nil, err
	}
	createdAt := owner.endpoint.clock().UTC().Truncate(time.Second)
	channel, err = flight.prefix.register(attempt, func(hello ardp.Hello, class uint8) ([]byte, error) {
		return owner.presentRegistrationToken(flight, hello, class)
	}, request)
	if err != nil {
		return nil, err
	}
	select {
	case <-channel.Done():
		return nil, errors.New("text Publisher registration ended before handover")
	default:
	}
	return introduction.NewRegistration(createdAt, channel, receiver, request, cancel), nil
}

func (owner *dutyContext) finishRegistration(ctx context.Context, flight *registrationFlight, registered *introduction.Registration, channel *client.ClosedIntroductionRegistration, outcome error) (*introduction.Registration, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	defer close(flight.done)
	current := owner.publication.pair.FinishOpeningLocked(flight)
	if outcome != nil || ctx.Err() != nil || flight.context.Err() != nil || !current || !flight.prefix.currentLocked(&owner.introduction.prefix) ||
		!owner.publication.pair.OpeningBaseLocked(flight.previous) || !owner.liveLocked(owner.endpoint, broker.Administration) {
		flight.cancel()
		cleanup := channel.Close()
		if errors.Is(outcome, client.ErrClosedSourceCleanup) || cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, outcome, cleanup)
			owner.closed = true
			owner.endpoint.failDutyContexts(owner.closeErr)
		}
		return nil, errors.Join(outcome, ctx.Err(), cleanup, errors.New("text Publisher registration did not complete"))
	}
	// Registration alone cannot switch published readiness or shorten its
	// predecessor. The verified Descriptor ACK establishes the overlap.
	if !owner.publication.pair.InstallLocked(flight.previous, registered) {
		flight.cancel()
		cleanup := channel.Close()
		if cleanup != nil {
			owner.closeErr = errors.Join(owner.closeErr, cleanup)
			owner.closed = true
			owner.endpoint.failDutyContexts(cleanup)
		}
		return nil, errors.Join(cleanup, errors.New("text Publisher registration owner changed before install"))
	}
	owner.publication.signalRegistrationsLocked()
	return registered, nil
}

func (owner *dutyContext) presentRegistrationToken(flight *registrationFlight, hello ardp.Hello, class uint8) ([]byte, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	profile, now, err := owner.permissionProfileLocked()
	if err != nil || owner.surface != broker.Administration || !owner.publication.pair.OpeningCurrentLocked(flight) || !flight.prefix.currentLocked(&owner.introduction.prefix) || flight.context.Err() != nil || !owner.tokens.PermissionLocked().Present() ||
		class != 3 || hello.Purpose != ardp.PurposeIntroduction || hello.RecipientNodeID != flight.receiver || hello.NetworkID != profile.NetworkID || hello.ProfileDigest != profile.Digest ||
		hello.StateGeneration != profile.StateGeneration || hello.StateDigest != profile.StateDigest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.After(profile.NotAfter) {
		return nil, errors.New("text Publisher registration token authority unavailable")
	}
	receiver, _, err := flight.prefix.introductionRecipient()
	if err != nil || receiver != flight.receiver {
		return nil, errors.New("text Publisher registration recipient changed")
	}
	return owner.tokens.TakeTokenLocked(profile, now, hello, class, flight.context)
}

func (owner *dutyContext) withdrawIntroduction(ctx context.Context) error {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return errors.New("text Publisher registration unavailable")
	}
	owner.mu.Lock()
	registered := owner.publication.pair.PublicationTargetLocked()
	if registered == nil || owner.resolution.BusyLocked() || owner.publication.pair.OpeningInProgressLocked() || owner.publication.pair.WithdrawalInProgressLocked() || !owner.liveLocked(owner.endpoint, broker.Administration) {
		owner.mu.Unlock()
		return errors.New("text Publisher registration absent or ending")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &operationFlight{owner: owner, context: attempt, cancelOperation: cancel, done: make(chan struct{})}
	if !owner.publication.pair.ReserveWithdrawalLocked(flight) {
		cancel()
		owner.mu.Unlock()
		return errors.New("text Publisher registration absent or ending")
	}
	owner.mu.Unlock()
	owner.publication.stopRefresh()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); cancel() })
	err := registered.Withdraw(attempt)
	cancel()
	if !stop() {
		<-interrupted
	}
	registered.Cancel()
	cleanup := registered.Close()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	previous := owner.publication.pair.CompleteWithdrawalLocked(flight, registered)
	owner.publication.signalRegistrationsLocked()
	if previous != nil {
		previous.Cancel()
		cleanup = errors.Join(cleanup, previous.Close())
	}
	if cleanup != nil {
		owner.closeErr = errors.Join(owner.closeErr, cleanup)
		owner.closed = true
		owner.endpoint.failDutyContexts(cleanup)
	}
	close(flight.done)
	return errors.Join(err, cleanup)
}

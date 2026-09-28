//go:build linux

package endpoint

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/service"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// joinedTransport retains the bounded Endpoint exchange until the Service
// owner has joined its physical transport. Finishing setup must not cancel it.
type joinedTransport struct {
	job         *jobIdentity
	joined      *client.ClosedJoinedStream
	acquisition joinAcquisition
	net.Conn
	once    sync.Once
	revoked context.Context
	stop    context.CancelFunc
	finish  func(error) error
	err     error
}

type joinPrefix interface {
	DataJoinRecipient() ([32]byte, uint64, time.Time, error)
	Join(context.Context, client.ClosedTokenPresenter, client.ClosedJoinIntent) (*client.ClosedJoinedStream, error)
}

type joinAcquisition interface {
	joinPrefix
	CurrentLocked(*dutyContext) bool
	IssuancePrefixLocked(*dutyContext) (*source.Handle, bool)
	Release()
}

// sourceJoinAcquisition adapts the extracted Source acquisition to the root
// JOIN interface shared with the Responder family. The Context argument stays
// at the root: an extracted acquisition knows its own lifecycle but never the
// duty context.
type sourceJoinAcquisition struct {
	*source.JoinAcquisition
}

func (acquisition sourceJoinAcquisition) CurrentLocked(owner *dutyContext) bool {
	return acquisition.JoinAcquisition.CurrentLocked(&owner.source)
}

func (acquisition sourceJoinAcquisition) IssuancePrefixLocked(owner *dutyContext) (*source.Handle, bool) {
	return acquisition.JoinAcquisition.IssuancePrefixLocked(&owner.source)
}

func (transport *joinedTransport) AuthenticatedPeerRetired() bool {
	witness, ok := transport.Conn.(interface{ AuthenticatedPeerRetired() bool })
	return ok && witness.AuthenticatedPeerRetired()
}

func (transport *joinedTransport) Close() error {
	transport.once.Do(func() {
		if transport.job != nil && transport.job.qualification != nil {
			transport.job.qualification.ReleaseJoin(transport.joined)
		}
		retirement := transport.Conn.Close()
		if transport.revoked != nil && transport.revoked.Err() != nil && routeStopOnly(retirement) {
			retirement = nil
		}
		transport.stop()
		transport.err = transport.finish(retirement)
		if transport.acquisition != nil {
			transport.acquisition.Release()
			transport.acquisition = nil
		}
	})
	return transport.err
}

func routeStopOnly(err error) bool {
	if err == nil || err == client.ErrClosedSourceStopped {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !routeStopOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return routeStopOnly(wrapped)
	}
	return errors.Is(err, net.ErrClosed)
}

// openJoinedService consumes the initial protected Route and installs its
// bounded replacement owner before Application bytes become reachable.
func (owner *dutyContext) openJoinedService(ctx context.Context, job *jobIdentity, attempt *introductionAttempt) (_ *service.Stream, outcome error) {
	return owner.openJoinedServiceAfterSetup(ctx, job, attempt, nil)
}

func (owner *dutyContext) openJoinedServiceAfterSetup(ctx context.Context, job *jobIdentity, attempt *introductionAttempt, setupComplete func()) (_ *service.Stream, outcome error) {
	transport, err := owner.openJoinedTransportAfterSetup(ctx, job, attempt, setupComplete)
	if err != nil {
		return nil, err
	}
	return attempt.binding.openServiceStreamWithRecovery(ctx, transport, attempt.digest,
		owner.serviceRouteRecoveryOpener(job, attempt.binding))
}

// openJoinedTransport consumes one locally prepared or independently
// accepted capsule. Source submission and JOIN run concurrently; neither
// grants Service authority. Its returned transport joins the complete Route
// exchange when the Service Attachment releases it.
func (owner *dutyContext) openJoinedTransport(ctx context.Context, job *jobIdentity, attempt *introductionAttempt) (_ *joinedTransport, outcome error) {
	return owner.openJoinedTransportAfterSetup(ctx, job, attempt, nil)
}

func (owner *dutyContext) openJoinedTransportAfterSetup(ctx context.Context, job *jobIdentity, attempt *introductionAttempt, setupComplete func()) (_ *joinedTransport, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || attempt == nil || !attempt.binding.servesJob(owner, job) {
		return nil, errors.New("text JOIN owner unavailable")
	}
	if err := attempt.binding.current(); err != nil {
		return nil, err
	}
	// The caller owns only this opening attempt. Once the joined transport is
	// transferred, job loss stops Service work while the transport's retained
	// cleanup owner remains able to send terminal control and join the Route.
	lifetime, flight, detachCaller, finish, err := owner.beginServiceTransportExchange(ctx, job, owner.surface)
	if err != nil {
		return nil, err
	}
	joining, stop := context.WithCancel(lifetime)
	transferred := false
	var raw *client.ClosedJoinedStream
	var acquisition joinAcquisition
	defer func() {
		if !transferred {
			if raw != nil {
				outcome = errors.Join(outcome, raw.Close())
			}
			stop()
			outcome = finish(outcome)
			if attempt.plaintext.AttachmentGeneration == 1 {
				outcome = errors.Join(outcome, attempt.binding.releaseIntroductionRecovery())
			}
			if acquisition != nil {
				acquisition.Release()
			}
		}
	}()
	owner.mu.Lock()
	prefix := owner.source.CurrentLocked()
	if owner.surface == broker.Connection {
		if join := owner.source.AcquireJoinLocked(); join != nil {
			acquisition = sourceJoinAcquisition{join}
		}
	}
	if owner.surface == broker.Administration {
		acquisition = owner.responder.acquireJoinLocked(prefix)
	}
	live := !attempt.joined && acquisition != nil && owner.liveServiceJobLocked(job, owner.surface)
	if live {
		attempt.joined = true
	}
	owner.mu.Unlock()
	if !live {
		return nil, errors.New("text JOIN attempt already consumed or retired")
	}
	// Issuance has one live exchange per context. Prepare JOIN stock before
	// launching the independent submission and JOIN network operations.
	if err := owner.prepareJoinStock(joining, attempt, acquisition); err != nil {
		return nil, err
	}
	if owner.surface == broker.Connection {
		bounded, cancel := context.WithDeadline(joining, attempt.plaintext.Deadline)
		prepare := owner.prepareSubmissionStock
		if attempt.plaintext.AttachmentGeneration > 1 {
			prepare = owner.prepareRecoverySubmissionStock
		}
		_, _, err := prepare(bounded, prefix)
		cancel()
		if err != nil {
			return nil, err
		}
	}
	if attempt.plaintext.AttachmentGeneration == 1 && owner.surface == broker.Connection {
		if err := owner.refreshIntroduction(joining, job, attempt, prefix); err != nil {
			return nil, err
		}
	}
	if setupComplete != nil {
		setupComplete()
	}
	type joinedResult struct {
		stream *client.ClosedJoinedStream
		err    error
	}
	joined := make(chan joinedResult, 1)
	go func() {
		stream, err := owner.joinIntroduction(joining, job, attempt, acquisition)
		joined <- joinedResult{stream, err}
	}()
	submitted := make(chan error, 1)
	if owner.surface == broker.Connection {
		go func() { submitted <- owner.submitIntroduction(joining, job, attempt) }()
	} else {
		submitted <- nil
	}
	for range 2 {
		select {
		case result := <-joined:
			raw = result.stream
			outcome = errors.Join(outcome, result.err)
			joined = nil
		case err := <-submitted:
			outcome = errors.Join(outcome, err)
			submitted = nil
		}
		if outcome != nil {
			stop()
		}
	}
	if outcome != nil {
		return nil, outcome
	}
	if !detachCaller() {
		return nil, errors.Join(ctx.Err(), errors.New("text JOIN caller ended before stream transfer"))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !owner.retainJoinedTransport(job, attempt, flight, acquisition, raw) {
		return nil, errors.New("text JOIN owner ended before stream transfer")
	}
	transport := &joinedTransport{job: job, joined: raw, acquisition: acquisition, Conn: raw, revoked: job.context, stop: stop, finish: finish}
	transferred = true
	return transport, nil
}

func (owner *dutyContext) prepareJoinStock(ctx context.Context, attempt *introductionAttempt, prefix joinAcquisition) error {
	node, generation, until, err := prefix.DataJoinRecipient()
	facts := attempt.plaintext
	if err != nil || node != facts.RendezvousNode || generation != facts.RendezvousDutyGeneration || facts.Deadline.After(until) {
		return errors.Join(err, errors.New("text JOIN capsule recipient changed"))
	}
	bounded, cancel := context.WithDeadline(ctx, facts.Deadline)
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	stocked := err == nil && prefix.CurrentLocked(owner) &&
		owner.tokens.Permission.StockCountFor(profile.Digest, node, 2) != 0
	owner.mu.Unlock()
	if err == nil && !stocked {
		err = owner.issueJoinTokens(bounded, [][32]byte{node}, 2, prefix,
			attempt.plaintext.AttachmentGeneration > 1)
	}
	cancel()
	if err != nil {
		return err
	}
	owner.mu.Lock()
	current := prefix.CurrentLocked(owner)
	owner.mu.Unlock()
	if !current {
		return errors.New("text JOIN Source acquisition changed during stock preparation")
	}
	return nil
}

func (owner *dutyContext) joinIntroduction(ctx context.Context, job *jobIdentity, attempt *introductionAttempt, prefix joinAcquisition) (*client.ClosedJoinedStream, error) {
	facts := attempt.plaintext
	node, generation, _, err := prefix.DataJoinRecipient()
	if err != nil || node != facts.RendezvousNode || generation != facts.RendezvousDutyGeneration {
		return nil, errors.Join(err, errors.New("text JOIN recipient changed before opening"))
	}
	owner.mu.Lock()
	profile, _, err := owner.permissionProfileLocked()
	owner.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := attempt.binding.current(); err != nil {
		return nil, err
	}
	return prefix.Join(ctx, func(hello ardp.Hello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		current, now, err := owner.permissionProfileLocked()
		if err != nil || current != profile || current.Digest != facts.ProfileDigest || !prefix.CurrentLocked(owner) || !owner.liveServiceJobLocked(job, owner.surface) || ctx.Err() != nil || !now.Before(facts.Deadline) || class != 2 || hello.Purpose != ardp.PurposeDataJoin || hello.RecipientNodeID != node || hello.RecipientDutyGeneration != generation || hello.NetworkID != current.NetworkID || hello.StateGeneration != current.StateGeneration || hello.StateDigest != current.StateDigest || hello.ProfileDigest != current.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.Unix() > facts.WorkSafetyNotAfter {
			return nil, errors.New("text JOIN token authority changed")
		}
		return owner.tokens.TakeTokenLocked(current, now, hello, class, ctx)
	}, client.ClosedJoinIntent{Secret: facts.JoinSecret, Context: facts.HandshakeContext, SetupDeadline: facts.Deadline, WorkDeadline: time.Unix(facts.WorkSafetyNotAfter, 0).UTC()})
}

func (owner *dutyContext) retainJoinedTransport(job *jobIdentity, attempt *introductionAttempt,
	flight *introduction.Exchange, acquisition joinAcquisition, joined *client.ClosedJoinedStream) bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if attempt == nil || !attempt.binding.servesJob(owner, job) ||
		acquisition == nil || !acquisition.CurrentLocked(owner) || joined == nil ||
		!owner.retainServiceTransportExchangeLocked(job, flight) {
		return false
	}
	if job.qualification != nil {
		job.qualification.RetainJoin(joined)
	}
	return true
}

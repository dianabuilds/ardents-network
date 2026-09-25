//go:build linux

package endpoint

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// textJoinedTransport retains the bounded Endpoint exchange until the Service
// owner has joined its physical transport. Finishing setup must not cancel it.
type textJoinedTransport struct {
	job         *textJobIdentity
	joined      *route.ClosedJoinedStream
	acquisition textJoinAcquisition
	net.Conn
	once    sync.Once
	revoked context.Context
	stop    context.CancelFunc
	finish  func(error) error
	err     error
}

type textJoinPrefix interface {
	dataJoinRecipient() ([32]byte, uint64, time.Time, error)
	join(context.Context, route.ClosedTokenPresenter, route.ClosedJoinIntent) (*route.ClosedJoinedStream, error)
}

type textJoinAcquisition interface {
	textJoinPrefix
	currentLocked(*textContext) bool
	issuancePrefixLocked(*textContext) (*textSourceHandle, bool)
	release()
}

func (transport *textJoinedTransport) AuthenticatedPeerRetired() bool {
	witness, ok := transport.Conn.(interface{ AuthenticatedPeerRetired() bool })
	return ok && witness.AuthenticatedPeerRetired()
}

func (transport *textJoinedTransport) Close() error {
	transport.once.Do(func() {
		if transport.job != nil && transport.job.qualification != nil {
			transport.job.qualification.ReleaseJoin(transport.joined)
		}
		retirement := transport.Conn.Close()
		if transport.revoked != nil && transport.revoked.Err() != nil && textRouteStopOnly(retirement) {
			retirement = nil
		}
		transport.stop()
		transport.err = transport.finish(retirement)
		if transport.acquisition != nil {
			transport.acquisition.release()
			transport.acquisition = nil
		}
	})
	return transport.err
}

func textRouteStopOnly(err error) bool {
	if err == nil || err == route.ErrClosedSourceStopped {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !textRouteStopOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return textRouteStopOnly(wrapped)
	}
	return errors.Is(err, net.ErrClosed)
}

// openTextJoinedService consumes the initial protected Route and installs its
// bounded replacement owner before Application bytes become reachable.
func (owner *textContext) openTextJoinedService(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt) (_ *textServiceStream, outcome error) {
	return owner.openTextJoinedServiceAfterSetup(ctx, job, attempt, nil)
}

func (owner *textContext) openTextJoinedServiceAfterSetup(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt, setupComplete func()) (_ *textServiceStream, outcome error) {
	transport, err := owner.openTextJoinedTransportAfterSetup(ctx, job, attempt, setupComplete)
	if err != nil {
		return nil, err
	}
	return attempt.binding.openTextServiceStreamWithRecovery(ctx, transport, attempt.digest,
		owner.textServiceRouteRecoveryOpener(job, attempt.binding))
}

// openTextJoinedTransport consumes one locally prepared or independently
// accepted capsule. Source submission and JOIN run concurrently; neither
// grants Service authority. Its returned transport joins the complete Route
// exchange when the Service Attachment releases it.
func (owner *textContext) openTextJoinedTransport(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt) (_ *textJoinedTransport, outcome error) {
	return owner.openTextJoinedTransportAfterSetup(ctx, job, attempt, nil)
}

func (owner *textContext) openTextJoinedTransportAfterSetup(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt, setupComplete func()) (_ *textJoinedTransport, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || attempt == nil || !attempt.binding.servesJob(owner, job) {
		return nil, errors.New("text JOIN owner unavailable")
	}
	if err := attempt.binding.current(); err != nil {
		return nil, err
	}
	// The caller owns only this opening attempt. Once the joined transport is
	// transferred, job loss stops Service work while the transport's retained
	// cleanup owner remains able to send terminal control and join the Route.
	lifetime, flight, detachCaller, finish, err := owner.beginTextServiceTransportExchange(ctx, job, owner.surface)
	if err != nil {
		return nil, err
	}
	joining, stop := context.WithCancel(lifetime)
	transferred := false
	var raw *route.ClosedJoinedStream
	var acquisition textJoinAcquisition
	defer func() {
		if !transferred {
			if raw != nil {
				outcome = errors.Join(outcome, raw.Close())
			}
			stop()
			outcome = finish(outcome)
			if attempt.plaintext.AttachmentGeneration == 1 {
				outcome = errors.Join(outcome, attempt.binding.releaseTextIntroductionRecovery())
			}
			if acquisition != nil {
				acquisition.release()
			}
		}
	}()
	owner.mu.Lock()
	source := owner.currentTextSourceLocked()
	if owner.surface == broker.Connection {
		acquisition = owner.source.acquireJoinLocked()
	}
	if owner.surface == broker.Administration {
		acquisition = owner.responder.acquireJoinLocked(source)
	}
	live := !attempt.joined && acquisition != nil && owner.liveTextServiceJobLocked(job, owner.surface)
	if live {
		attempt.joined = true
	}
	owner.mu.Unlock()
	if !live {
		return nil, errors.New("text JOIN attempt already consumed or retired")
	}
	// Issuance has one live exchange per context. Prepare JOIN stock before
	// launching the independent submission and JOIN network operations.
	if err := owner.prepareTextJoinStock(joining, attempt, acquisition); err != nil {
		return nil, err
	}
	if owner.surface == broker.Connection {
		bounded, cancel := context.WithDeadline(joining, attempt.plaintext.Deadline)
		prepare := owner.prepareTextSubmissionStock
		if attempt.plaintext.AttachmentGeneration > 1 {
			prepare = owner.prepareTextRecoverySubmissionStock
		}
		_, _, err := prepare(bounded, source)
		cancel()
		if err != nil {
			return nil, err
		}
	}
	if attempt.plaintext.AttachmentGeneration == 1 && owner.surface == broker.Connection {
		if err := owner.refreshTextIntroduction(joining, job, attempt, source); err != nil {
			return nil, err
		}
	}
	if setupComplete != nil {
		setupComplete()
	}
	type joinedResult struct {
		stream *route.ClosedJoinedStream
		err    error
	}
	joined := make(chan joinedResult, 1)
	go func() {
		stream, err := owner.joinTextIntroduction(joining, job, attempt, acquisition)
		joined <- joinedResult{stream, err}
	}()
	submitted := make(chan error, 1)
	if owner.surface == broker.Connection {
		go func() { submitted <- owner.submitTextIntroduction(joining, job, attempt) }()
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
	if !owner.retainTextJoinedTransport(job, attempt, flight, acquisition, raw) {
		return nil, errors.New("text JOIN owner ended before stream transfer")
	}
	transport := &textJoinedTransport{job: job, joined: raw, acquisition: acquisition, Conn: raw, revoked: job.context, stop: stop, finish: finish}
	transferred = true
	return transport, nil
}

func (owner *textContext) prepareTextJoinStock(ctx context.Context, attempt *textIntroductionAttempt, prefix textJoinAcquisition) error {
	node, generation, until, err := prefix.dataJoinRecipient()
	facts := attempt.plaintext
	if err != nil || node != facts.RendezvousNode || generation != facts.RendezvousDutyGeneration || facts.Deadline.After(until) {
		return errors.Join(err, errors.New("text JOIN capsule recipient changed"))
	}
	bounded, cancel := context.WithDeadline(ctx, facts.Deadline)
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	stocked := err == nil && prefix.currentLocked(owner) &&
		owner.permission.stockCountFor(profile.Digest, node, 2) != 0
	owner.mu.Unlock()
	if err == nil && !stocked {
		err = owner.issueTextJoinTokens(bounded, [][32]byte{node}, 2, prefix,
			attempt.plaintext.AttachmentGeneration > 1)
	}
	cancel()
	if err != nil {
		return err
	}
	owner.mu.Lock()
	current := prefix.currentLocked(owner)
	owner.mu.Unlock()
	if !current {
		return errors.New("text JOIN Source acquisition changed during stock preparation")
	}
	return nil
}

func (owner *textContext) joinTextIntroduction(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt, prefix textJoinAcquisition) (*route.ClosedJoinedStream, error) {
	facts := attempt.plaintext
	node, generation, _, err := prefix.dataJoinRecipient()
	if err != nil || node != facts.RendezvousNode || generation != facts.RendezvousDutyGeneration {
		return nil, errors.Join(err, errors.New("text JOIN recipient changed before opening"))
	}
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	owner.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := attempt.binding.current(); err != nil {
		return nil, err
	}
	return prefix.join(ctx, func(hello ardp.Hello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		current, now, err := owner.textPermissionProfileLocked()
		if err != nil || current != profile || current.Digest != facts.ProfileDigest || !prefix.currentLocked(owner) || !owner.liveTextServiceJobLocked(job, owner.surface) || ctx.Err() != nil || !now.Before(facts.Deadline) || class != 2 || hello.Purpose != ardp.PurposeDataJoin || hello.RecipientNodeID != node || hello.RecipientDutyGeneration != generation || hello.NetworkID != current.NetworkID || hello.StateGeneration != current.StateGeneration || hello.StateDigest != current.StateDigest || hello.ProfileDigest != current.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.Unix() > facts.WorkSafetyNotAfter {
			return nil, errors.New("text JOIN token authority changed")
		}
		return owner.takeTextTokenLocked(current, now, hello, class, ctx)
	}, route.ClosedJoinIntent{Secret: facts.JoinSecret, Context: facts.HandshakeContext, SetupDeadline: facts.Deadline, WorkDeadline: time.Unix(facts.WorkSafetyNotAfter, 0).UTC()})
}

func (owner *textContext) retainTextJoinedTransport(job *textJobIdentity, attempt *textIntroductionAttempt,
	flight *textIntroductionExchange, acquisition textJoinAcquisition, joined *route.ClosedJoinedStream) bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if attempt == nil || !attempt.binding.servesJob(owner, job) ||
		acquisition == nil || !acquisition.currentLocked(owner) || joined == nil ||
		!owner.retainTextServiceTransportExchangeLocked(job, flight) {
		return false
	}
	if job.qualification != nil {
		job.qualification.RetainJoin(joined)
	}
	return true
}

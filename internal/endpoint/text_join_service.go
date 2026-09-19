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
)

// textJoinedTransport retains the bounded Endpoint exchange until the Service
// owner has joined its physical transport. Finishing setup must not cancel it.
type textJoinedTransport struct {
	job    *textJobIdentity
	joined *route.ClosedJoinedStream
	net.Conn
	once    sync.Once
	revoked context.Context
	stop    context.CancelFunc
	finish  func(error) error
	err     error
}

func (transport *textJoinedTransport) Close() error {
	transport.once.Do(func() {
		if transport.job != nil {
			owner := transport.job.owner
			owner.mu.Lock()
			delete(transport.job.qualificationJoins, transport.joined)
			owner.mu.Unlock()
		}
		retirement := transport.Conn.Close()
		if transport.revoked != nil && transport.revoked.Err() != nil && textRouteStopOnly(retirement) {
			retirement = nil
		}
		transport.stop()
		transport.err = transport.finish(retirement)
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
	transport, err := owner.openTextJoinedTransport(ctx, job, attempt)
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
	if owner == nil || ctx == nil || ctx.Err() != nil || attempt == nil || attempt.binding == nil || attempt.binding.owner != owner || attempt.binding.job != job {
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
		}
	}()
	owner.mu.Lock()
	prefix := owner.prefix
	if owner.surface == broker.Administration {
		prefix = owner.responder.prefix
	}
	live := !attempt.joined && prefix != nil && owner.liveTextServiceJobLocked(job, owner.surface)
	if live {
		attempt.joined = true
	}
	owner.mu.Unlock()
	if !live {
		return nil, errors.New("text JOIN attempt already consumed or retired")
	}
	// Issuance has one live exchange per context. Prepare JOIN stock before
	// launching the independent submission and JOIN network operations.
	if err := owner.prepareTextJoinStock(joining, attempt, prefix); err != nil {
		return nil, err
	}
	if owner.surface == broker.Connection {
		bounded, cancel := context.WithDeadline(joining, attempt.plaintext.Deadline)
		prepare := owner.prepareTextSubmissionStock
		if attempt.plaintext.AttachmentGeneration > 1 {
			prepare = owner.prepareTextRecoverySubmissionStock
		}
		_, _, err := prepare(bounded, prefix)
		cancel()
		if err != nil {
			return nil, err
		}
	}
	if attempt.plaintext.AttachmentGeneration == 1 && owner.surface == broker.Connection {
		if job.qualificationAcquireIntroduction != nil {
			if err := job.qualificationAcquireIntroduction(joining); err != nil {
				return nil, err
			}
		}
		if err := owner.refreshTextIntroduction(joining, job, attempt, prefix); err != nil {
			return nil, err
		}
	}
	type joinedResult struct {
		stream *route.ClosedJoinedStream
		err    error
	}
	joined := make(chan joinedResult, 1)
	go func() {
		stream, err := owner.joinTextIntroduction(joining, job, attempt, prefix)
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
	if !owner.retainTextServiceTransportExchange(job, flight) {
		return nil, errors.New("text JOIN owner ended before stream transfer")
	}
	owner.mu.Lock()
	if job.qualification != nil {
		if job.qualificationJoins == nil {
			job.qualificationJoins = make(map[*route.ClosedJoinedStream]struct{})
		}
		job.qualificationJoins[raw] = struct{}{}
	}
	owner.mu.Unlock()
	transport := &textJoinedTransport{job: job, joined: raw, Conn: raw, revoked: job.context, stop: stop, finish: finish}
	transferred = true
	return transport, nil
}

func (owner *textContext) prepareTextJoinStock(ctx context.Context, attempt *textIntroductionAttempt, prefix *route.ClosedSourcePrefix) error {
	node, generation, until, err := prefix.DataJoinRecipient()
	facts := attempt.plaintext
	if err != nil || node != facts.RendezvousNode || generation != facts.RendezvousDutyGeneration || facts.Deadline.After(until) {
		return errors.Join(err, errors.New("text JOIN capsule recipient changed"))
	}
	bounded, cancel := context.WithDeadline(ctx, facts.Deadline)
	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	stocked := false
	if err == nil && owner.permission != nil {
		for _, stock := range owner.permission.stock {
			if stock.challenge.ReceiverNodeID == node && stock.challenge.ProfileDigest == profile.Digest && stock.challenge.Class == 2 && stock.challenge.WindowStart == owner.permission.accepted.NotBefore && len(stock.tokens) != 0 {
				stocked = true
			}
		}
	}
	owner.mu.Unlock()
	if err == nil && !stocked {
		if attempt.plaintext.AttachmentGeneration > 1 {
			err = owner.issueTextRecoveryTokens(bounded, [][32]byte{node}, 2)
		} else {
			err = owner.issueTextTokens(bounded, [][32]byte{node}, 2)
		}
	}
	cancel()
	if err != nil {
		return err
	}
	return nil
}

func (owner *textContext) joinTextIntroduction(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt, prefix *route.ClosedSourcePrefix) (*route.ClosedJoinedStream, error) {
	facts := attempt.plaintext
	node, generation, _, err := prefix.DataJoinRecipient()
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
	return prefix.Join(ctx, func(hello route.ClosedHello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		current, now, err := owner.textPermissionProfileLocked()
		selected := owner.prefix
		if owner.surface == broker.Administration {
			selected = owner.responder.prefix
		}
		if err != nil || current != profile || current.Digest != facts.ProfileDigest || selected != prefix || !owner.liveTextServiceJobLocked(job, owner.surface) || ctx.Err() != nil || !now.Before(facts.Deadline) || class != 2 || hello.Purpose != route.ClosedPurposeDataJoin || hello.RecipientNodeID != node || hello.RecipientDutyGeneration != generation || hello.NetworkID != current.NetworkID || hello.StateGeneration != current.StateGeneration || hello.StateDigest != current.StateDigest || hello.ProfileDigest != current.Digest || hello.ChannelNonce == [32]byte{} || !now.Before(hello.Deadline) || hello.Deadline.Unix() > facts.WorkSafetyNotAfter {
			return nil, errors.New("text JOIN token authority changed")
		}
		return owner.takeTextTokenLocked(current, now, hello, class, ctx)
	}, route.ClosedJoinIntent{Secret: facts.JoinSecret, Context: facts.HandshakeContext, SetupDeadline: facts.Deadline, WorkDeadline: time.Unix(facts.WorkSafetyNotAfter, 0).UTC()})
}

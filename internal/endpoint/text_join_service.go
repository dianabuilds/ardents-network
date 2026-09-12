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
	net.Conn
	once   sync.Once
	stop   context.CancelFunc
	finish func(error) error
	err    error
}

func (transport *textJoinedTransport) Close() error {
	transport.once.Do(func() {
		transport.err = transport.Conn.Close()
		transport.stop()
		transport.err = transport.finish(transport.err)
	})
	return transport.err
}

// openTextJoinedService consumes one locally prepared or independently accepted
// capsule. Source submission and JOIN run concurrently; neither grants Service
// authority. Only the existing Service authentication can expose Application I/O.
func (owner *textContext) openTextJoinedService(ctx context.Context, job *textJobIdentity, attempt *textIntroductionAttempt) (_ *textServiceStream, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || attempt == nil || attempt.binding == nil || attempt.binding.owner != owner || attempt.binding.job != job {
		return nil, errors.New("text JOIN owner unavailable")
	}
	if err := attempt.binding.current(); err != nil {
		return nil, err
	}
	lifetime, finish, err := owner.beginTextIntroductionExchange(ctx, job, owner.surface)
	if err != nil {
		return nil, err
	}
	joining, stop := context.WithCancel(lifetime)
	transferred := false
	defer func() {
		if !transferred {
			stop()
			outcome = finish(outcome)
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
		_, _, err := owner.prepareTextSubmissionStock(bounded, prefix)
		cancel()
		if err != nil {
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
	var raw *route.ClosedJoinedStream
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
		if raw != nil {
			outcome = errors.Join(outcome, raw.Close())
		}
		return nil, outcome
	}
	transport := &textJoinedTransport{Conn: raw, stop: stop, finish: finish}
	transferred = true
	// Keep the caller context independent of exchange cleanup: a normal transport
	// close must not relabel a completed Service as caller cancellation.
	return attempt.binding.openTextServiceStream(ctx, transport, attempt.digest)
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
		err = owner.issueTextTokens(bounded, [][32]byte{node}, 2)
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

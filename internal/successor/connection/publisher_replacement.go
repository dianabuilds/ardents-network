package connection

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// PublisherReplacement is an authenticated, still setup-bounded proposal. It
// exposes no Application I/O. Commit checks the original owners and transfers
// it only after the old physical actors join; Close joins a rejected proposal.
type PublisherReplacement struct {
	self                                   *PublisherReplacement
	stream                                 *Stream
	ticket                                 *recoveryProposal
	service                                *instance.ServiceAttachment // Stream.mu publishes ownership.
	physical                               *join.Joined
	acquisition                            *join.JoinAcquisition
	opening                                *instance.Opening
	peer                                   wire.Continuity
	attachment                             [32]byte
	previous                               *attachmentRecords
	revision                               uint64
	ctx                                    context.Context
	caller                                 context.Context
	cancel                                 context.CancelFunc
	stopCaller, stopInterruption           func() bool
	callerDone, interruptionDone, prepared chan struct{}
	interruptErr                           error
	setupOnce, closeOnce                   sync.Once
	commitMu                               sync.Mutex
	failure, closeErr                      error
	closed                                 bool // Stream.mu seals the proposal.
	transferred                            bool // Accepted physical ownership belongs to Stream.
	route                                  *publisherRouteAcquisition
}

// PreparePublisherReplacement reserves a proposal before acquiring or spending
// a fresh genuine Responder JOIN, then authenticates it
// under the SAME original Stream/Instance lease and continuity key. Request
// generation and peer offsets cannot mutate the existing logical ledgers.
// The original logical stream must still be live, including physical gaps.
// The public JOIN configuration must match the actual private opening exactly;
// Route independently checks its recipient. Refusal joins all admitted physical
// work before retiring the spent ticket, including failed JOIN/TLS setup.
func (stream *Stream) PreparePublisherReplacement(ctx context.Context, opening *instance.Opening, config join.JoinConfig) (_ *PublisherReplacement, result error) {
	if stream == nil || stream.self != stream || stream.ctx == nil || stream.owner == nil || ctx == nil || opening == nil {
		return nil, errIntegrity
	}
	var proposal *PublisherReplacement
	var key [32]byte
	err := stream.commit(func() error {
		if stream.owner.role != wire.RolePublisher || stream.publisherRoute == nil || stream.key == [32]byte{} || stream.publisherProposal != nil || stream.send.generation != stream.receive.generation || stream.send.generation != stream.terminal.generation {
			return errIntegrity
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if stream.recovery == nil {
			var err error
			stream.recovery, err = newRecoveryEpisode(stream.send.generation, stream.lastProgress, time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0))
			if err != nil {
				return err
			}
		}
		ticket, err := stream.recovery.begin(stream.send.generation, time.Now())
		if err != nil {
			return err
		}
		child, cancel := context.WithDeadline(stream.ctx, ticket.deadline)
		proposal = &PublisherReplacement{stream: stream, ticket: ticket, opening: opening, ctx: child, caller: ctx, cancel: cancel,
			callerDone: make(chan struct{}), interruptionDone: make(chan struct{}), prepared: make(chan struct{})}
		proposal.self = proposal
		proposal.stopCaller = context.AfterFunc(ctx, func() { defer close(proposal.callerDone); cancel() })
		stream.publisherProposal = proposal
		stream.prepareUsers.Add(1)
		key = stream.key
		return nil
	})
	if proposal == nil {
		return nil, err
	}
	child, ticket := proposal.ctx, proposal.ticket
	defer clear(key[:])
	defer stream.prepareUsers.Done()
	defer func() {
		stream.mu.Lock()
		proposal.failure = result
		stream.mu.Unlock()
		close(proposal.prepared)
		if result != nil {
			result = errors.Join(result, proposal.Close())
		}
	}()
	if err != nil {
		return nil, err
	}
	check := func() error {
		return stream.commit(func() error {
			if proposal.closed || stream.publisherProposal != proposal || stream.recovery.active != ticket || !time.Now().Before(ticket.deadline) {
				return errRecoveryExhausted
			}
			return errors.Join(ctx.Err(), child.Err())
		})
	}
	violation := func(cause error) error {
		err := errors.Join(errIntegrity, cause)
		stream.mu.Lock()
		stream.failLocked(err)
		stream.mu.Unlock()
		return err
	}
	if err := check(); err != nil {
		return nil, err
	}
	request, digest, err := opening.Request(stream.owner.operation.Context())
	if err != nil {
		return nil, err
	}
	stream.mu.Lock()
	revision := stream.recipientRevision
	stream.mu.Unlock()
	if request.AttachmentGeneration != ticket.generation || request.Deadline.After(ticket.deadline) || request.Revision < revision {
		return nil, errIntegrity
	}
	proposal.revision = request.Revision
	identity := stream.owner.identity
	if request.Network != identity.Network || request.Target != identity.Target || request.PublicationDigest != identity.Publication ||
		request.ProfileDigest != identity.Profile || request.ConnectionNonce != identity.ConnectionNonce || request.InitiatorBinding != identity.InitiatorBinding ||
		request.WorkSafetyNotAfter != identity.WorkSafetyNotAfter || request.WorkSafetyMaximum != identity.WorkSafetyMaximum || request.NoNewRecoveryAfter != identity.NoNewRecoveryAfter ||
		config.Secret != request.JoinSecret || config.Context != request.HandshakeContext || !config.SetupDeadline.Equal(request.Deadline) ||
		!config.Deadline.Equal(time.Unix(identity.WorkSafetyNotAfter, 0)) || config.Duty.NodeID != request.RendezvousNode || config.Duty.RecordGeneration != request.RendezvousDutyGeneration {
		return nil, errIntegrity
	}
	proposal.route, err = stream.publisherRoute.acquire(child)
	if err != nil {
		return nil, err
	}
	retiredWork := stream.publisherRoute.responder.RetiredWork()
	stream.mu.Lock()
	if stream.recoveryLoss == nil {
		stream.recoveryLoss = retiredWork
	}
	stream.mu.Unlock()
	if err := proposal.route.checkRecipient(config); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	// The acquisition's caller is the original logical lifetime, not the
	// finite attempt. Only setup interruption uses the proposal deadline.
	acquisition, err := proposal.route.join(stream.owner.context())
	if err != nil {
		return nil, err
	}
	proposal.acquisition = acquisition
	proposal.stopInterruption = context.AfterFunc(child, func() {
		defer close(proposal.interruptionDone)
		proposal.interruptErr = acquisition.Close()
	})
	physical, err := acquisition.Join(stream.owner.context(), config)
	if err != nil {
		return nil, err
	}
	proposal.physical = physical
	if err := check(); err != nil {
		return nil, err
	}
	service, err := stream.owner.lease.BeginReplacement(opening, physical)
	if err != nil {
		return nil, err
	}
	stream.mu.Lock()
	proposal.service = service
	stream.mu.Unlock()
	_, proposal.attachment, err = wire.Contexts(stream.owner.identity, digest, ticket.generation)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := service.Handshake(child); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	evidence, err := service.Evidence()
	if err != nil {
		return nil, err
	}
	record, err := wire.Read(service)
	if err := errors.Join(err, check()); err != nil {
		if errors.Is(err, wire.ErrInvalidRecord) {
			return nil, violation(err)
		}
		return nil, err
	}
	if record.Continuity == nil {
		return nil, violation(nil)
	}
	receipt, err := wire.VerifyReplacement(evidence, key, *record.Continuity, wire.RoleInitiator, ticket.generation, stream.owner.logical)
	if err != nil {
		return nil, violation(err)
	}
	peer, err := receipt.Consume(evidence, wire.RoleInitiator, ticket.generation, stream.owner.logical, wire.ContinuityCommitment(key))
	if err != nil {
		return nil, violation(err)
	}
	// The new peer is authenticated. Freeze and physically join the old actors
	// before publishing offsets or replay decisions; pending local bytes survive.
	err = stream.commit(func() error {
		proposal.previous = stream.records
		return stream.startRecoveryLocked(proposal.previous)
	})
	if err != nil {
		return nil, err
	}
	select {
	case <-child.Done():
		return nil, child.Err()
	case <-proposal.previous.retired:
	}
	if err := check(); err != nil {
		return nil, err
	}
	var sendBase, sendEnd, receiveNext uint64
	err = stream.commit(func() error {
		if err := errors.Join(stream.send.checkReplacement(ticket.generation, peer.ReceiveNext), stream.receive.checkReplacement(ticket.generation, peer.SendBase, peer.SendEnd)); err != nil {
			return err
		}
		sendBase, sendEnd, receiveNext = stream.send.base, stream.send.end, stream.receive.next
		return nil
	})
	if err != nil {
		if errors.Is(err, errIntegrity) {
			return nil, violation(err)
		}
		return nil, err
	}
	_, exporter := evidence.Secrets() // Its fresh key is deliberately not used.
	local, err := wire.NewContinuity(key, wire.RolePublisher, ticket.generation, sendBase, sendEnd, receiveNext, stream.owner.logical, exporter)
	if err != nil {
		return nil, err
	}
	response, err := wire.NewReplacementResponse(receipt, key, local)
	if err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	if err := service.SendReplacement(child, response); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	proposal.peer = peer
	return proposal, nil
}

func (proposal *PublisherReplacement) joinSetup() {
	proposal.setupOnce.Do(func() {
		if !proposal.stopCaller() {
			<-proposal.callerDone
		}
		if proposal.stopInterruption != nil && !proposal.stopInterruption() {
			<-proposal.interruptionDone
		}
	})
}

// Close joins only this rejected proposal, never its original logical lease
// or another accepted Attachment. The spent ticket and actual cleanup survive.
func (proposal *PublisherReplacement) Close() error {
	if proposal == nil {
		return nil
	}
	if proposal.self != proposal || proposal.stream == nil || proposal.cancel == nil || proposal.prepared == nil {
		return errIntegrity
	}
	proposal.closeOnce.Do(func() {
		stream := proposal.stream
		stream.mu.Lock()
		if proposal.transferred {
			stream.mu.Unlock()
			proposal.route.close()
			return
		}
		proposal.closed = true
		stream.mu.Unlock()
		proposal.cancel()
		<-proposal.prepared
		// Preparation may still be acquiring its first physical child when
		// Close seals it. Join after its final publication so no later callback
		// escapes a setupOnce that already ran with an absent interruption.
		proposal.joinSetup()
		var retirement, physical error
		if proposal.service != nil {
			retirement = proposal.service.Close()
			var joined bool
			physical, joined = proposal.service.CleanupResult()
			if !joined {
				physical = errors.Join(physical, errIntegrity)
			}
		} else if proposal.acquisition != nil {
			retirement = proposal.acquisition.Close()
			var joined bool
			physical, joined = proposal.acquisition.CleanupResult()
			if !joined {
				physical = errors.Join(physical, errIntegrity)
			}
		}
		stream.mu.Lock()
		failure := errors.Join(proposal.failure, proposal.ctx.Err(), proposal.interruptErr, retirement, physical)
		proposal.closeErr = stream.recovery.reject(proposal.ticket, failure)
		stream.recoveryCleanup = errors.Join(stream.recoveryCleanup, physical)
		if stream.publisherProposal == proposal {
			stream.publisherProposal = nil
		}
		stream.mu.Unlock()
		stream.owner.operation.RetainCleanup(physical)
		proposal.route.close()
	})
	return proposal.closeErr
}

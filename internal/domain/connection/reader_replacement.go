package connection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/domain/route"
	"github.com/dianabuilds/ardents-network/internal/domain/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/domain/route/introduction"
	"github.com/dianabuilds/ardents-network/internal/domain/route/join"
	"github.com/dianabuilds/ardents-network/internal/domain/route/prefix"
)

// One finite original-key proposal owns all new physical resources until its
// exact generation commits. Neither setup cancellation nor rejected cleanup
// can reach an accepted Attachment or another Application operation.
type initiatorReplacement struct {
	stream                     *Stream
	ticket                     *transfer.RecoveryTicket
	previous                   *attachmentRecords
	ctx                        context.Context
	cancel                     context.CancelFunc
	slot                       uint8
	acquisition                *join.Acquisition
	sourceHandle               *initiatorRouteAcquisition
	attachment                 *initiatorAttachment
	peer                       wire.Continuity
	stopInterruption           func() bool
	interruptionDone, prepared chan struct{}
	interruptErr               error
	closed, transferred        bool // Stream.mu owns these seals.
	failure, closeErr          error
	setupOnce, closeOnce       sync.Once
}

func (stream *Stream) recoverInitiator(records *attachmentRecords) {
	defer stream.recoveryUsers.Done()
	select {
	case <-stream.ctx.Done():
		return
	case <-records.retired:
	}
	var last error
	for {
		proposal, err := stream.prepareInitiatorReplacement(records)
		if err == nil {
			err = proposal.commit()
		}
		if err == nil {
			return
		}
		last = errors.Join(last, err)
		err = stream.commit(func() error {
			if stream.records != records {
				return errIntegrity
			}
			if stream.recovery == nil || !stream.recovery.CanRetry(time.Now()) {
				return errors.Join(errRecoveryExhausted, last)
			}
			return nil
		})
		if err != nil {
			stream.mu.Lock()
			if !stream.closing && stream.ctx.Err() == nil && stream.records == records {
				stream.failLocked(recoveryFailure(records.recoveryCause, errors.Join(last, err)))
			}
			stream.mu.Unlock()
			return
		}
	}
}

func (stream *Stream) prepareInitiatorReplacement(records *attachmentRecords) (_ *initiatorReplacement, result error) {
	var proposal *initiatorReplacement
	var key [32]byte
	err := stream.commit(func() error {
		if stream.initiatorRoute == nil || stream.records != records || !records.retiring || stream.initiatorProposal != nil || stream.key == [32]byte{} || stream.recovery == nil {
			return errIntegrity
		}
		ticket, err := stream.recovery.Begin(records.generation, time.Now())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithDeadline(stream.ctx, ticket.Deadline())
		proposal = &initiatorReplacement{stream: stream, ticket: ticket, previous: records, ctx: ctx, cancel: cancel,
			interruptionDone: make(chan struct{}), prepared: make(chan struct{})}
		if ticket.IsRetry() {
			proposal.slot = 1
		}
		stream.initiatorProposal = proposal
		stream.prepareUsers.Add(1)
		key = stream.key
		return nil
	})
	if proposal == nil {
		return nil, err
	}
	ticket := proposal.ticket
	defer clear(key[:])
	defer stream.prepareUsers.Done()
	defer func() {
		stream.mu.Lock()
		proposal.failure = result
		stream.mu.Unlock()
		close(proposal.prepared)
		if result != nil {
			result = errors.Join(result, proposal.close())
		}
	}()
	if err != nil {
		return nil, err
	}
	binding, physical, err := stream.initiatorRoute.openReplacement(proposal)
	if err != nil {
		return nil, err
	}
	child, cancelPhysical := context.WithCancel(stream.owner.context())
	attachment := &initiatorAttachment{binding: binding, physical: physical, ctx: child, cancel: cancelPhysical, watcherDone: make(chan struct{})}
	attachment.self = attachment
	attachment.connection = newInstanceTLSClient(initiatorTLSBorrow{attachment: attachment}, binding.identity.Instance)
	proposal.attachment = attachment
	go attachment.watch()
	if err := physical.SetDeadline(binding.expires); err != nil {
		return nil, err
	}
	if err := attachment.enter(); err != nil {
		return nil, err
	}
	err = attachment.connection.HandshakeContext(proposal.ctx)
	attachment.users.Done()
	if err := errors.Join(err, proposal.check(), attachment.check()); err != nil {
		return nil, err
	}
	evidence, err := wire.BindTLS(attachment.connection, binding.attachment)
	if err != nil {
		return nil, err
	}
	attachment.evidence = evidence
	_, exporter := evidence.Secrets() // Retain the original logical key.
	stream.mu.Lock()
	offsets := stream.transfer.ReplayOffsets()
	sendBase, sendEnd, receiveNext := offsets.SendBase, offsets.SendEnd, offsets.ReceiveNext
	stream.mu.Unlock()
	local, err := wire.NewContinuity(key, wire.RoleInitiator, ticket.Generation(), sendBase, sendEnd, receiveNext, stream.owner.logical, exporter)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true)); err != nil {
		return nil, err
	}
	if err := wire.Write(attachment, wire.Record{Continuity: &local}); err != nil {
		return nil, err
	}
	record, err := wire.Read(attachment)
	if err != nil {
		if errors.Is(err, wire.ErrInvalidRecord) {
			proposal.failIntegrity(err)
		}
		return nil, err
	}
	if record.Continuity == nil || record.Continuity.Nonce == local.Nonce {
		return nil, proposal.failIntegrity(nil)
	}
	receipt, err := wire.VerifyReplacement(evidence, key, *record.Continuity, wire.RolePublisher, ticket.Generation(), stream.owner.logical)
	if err != nil {
		return nil, proposal.failIntegrity(err)
	}
	peer, err := receipt.Consume(evidence, wire.RolePublisher, ticket.Generation(), stream.owner.logical, wire.ContinuityCommitment(key))
	if err != nil {
		return nil, proposal.failIntegrity(err)
	}
	stream.mu.Lock()
	err = stream.transfer.CheckReplacement(ticket.Generation(), peer)
	stream.mu.Unlock()
	if err != nil {
		return nil, proposal.failIntegrity(err)
	}
	if err := errors.Join(proposal.check(), binding.check(true), attachment.check()); err != nil {
		return nil, err
	}
	if err := physical.SetDeadline(time.Unix(stream.owner.identity.WorkSafetyNotAfter, 0)); err != nil {
		return nil, err
	}
	attachment.mu.Lock()
	attachment.initialComplete = true
	attachment.mu.Unlock()
	proposal.peer = peer
	return proposal, nil
}

func (proposal *initiatorReplacement) check() error {
	stream := proposal.stream
	return stream.commit(func() error {
		if proposal.closed || stream.initiatorProposal != proposal || stream.recovery == nil || !stream.recovery.Owns(proposal.ticket) || !time.Now().Before(proposal.ticket.Deadline()) {
			return errRecoveryExhausted
		}
		return proposal.ctx.Err()
	})
}

func (proposal *initiatorReplacement) failIntegrity(cause error) error {
	err := errors.Join(errIntegrity, cause)
	proposal.stream.mu.Lock()
	proposal.stream.failLocked(err)
	proposal.stream.mu.Unlock()
	return err
}

func (proposal *initiatorReplacement) joinSetup() {
	proposal.setupOnce.Do(func() {
		if proposal.stopInterruption != nil && !proposal.stopInterruption() {
			<-proposal.interruptionDone
		}
	})
}

func (proposal *initiatorReplacement) close() error {
	proposal.closeOnce.Do(func() {
		stream := proposal.stream
		stream.mu.Lock()
		if proposal.transferred {
			stream.mu.Unlock()
			proposal.sourceHandle.close()
			return
		}
		proposal.closed = true
		stream.mu.Unlock()
		proposal.cancel()
		<-proposal.prepared
		proposal.joinSetup()
		var retirement, physical error
		if proposal.attachment != nil {
			retirement = proposal.attachment.Close()
			physical = proposal.attachment.cleanupErr
		} else if proposal.acquisition != nil {
			retirement = proposal.acquisition.Close()
			var joined bool
			physical, joined = proposal.acquisition.CleanupResult()
			if !joined {
				physical = errors.Join(physical, errIntegrity)
			}
		}
		stream.mu.Lock()
		proposal.closeErr = stream.recovery.Reject(proposal.ticket, errors.Join(proposal.failure, proposal.ctx.Err(), proposal.interruptErr, retirement, physical))
		stream.recoveryCleanup = errors.Join(stream.recoveryCleanup, physical)
		if stream.initiatorProposal == proposal {
			stream.initiatorProposal = nil
		}
		stream.mu.Unlock()
		stream.owner.operation.RetainCleanup(physical)
		proposal.sourceHandle.close()
	})
	return proposal.closeErr
}

// openReplacement uses the SAME original Source acquisition for this finite
// proposal's current lookup, recipient, fresh capsule and actual JOIN. The
// caller reserved its ticket before any acquisition or irreversible spend.
func (source *InitiatorSource) openReplacement(proposal *initiatorReplacement) (*InitiatorBinding, *join.Joined, error) {
	if err := proposal.check(); err != nil {
		return nil, nil, err
	}
	handle, err := source.scope.Acquire(proposal.ctx)
	if err != nil {
		return nil, nil, errors.Join(err, source.scope.RetiredWork())
	}
	proposal.sourceHandle = &initiatorRouteAcquisition{handle: handle}
	acquisition, err := join.AcquireScopedSourceJoin(proposal.stream.owner.context(), handle)
	if err != nil {
		return nil, nil, err
	}
	proposal.acquisition = acquisition
	proposal.stopInterruption = context.AfterFunc(proposal.ctx, func() {
		defer close(proposal.interruptionDone)
		proposal.interruptErr = acquisition.Close()
	})
	lookupEnd := proposal.ticket.Deadline().UTC().Truncate(time.Second)
	candidate, err := handle.LookupDescriptor(proposal.ctx, lookupEnd, source.target, source.config.History, source.config.Exclusions)
	if err != nil {
		return nil, nil, fmt.Errorf("connection replacement Descriptor lookup: %w", err)
	}
	descriptor, err := reachability.Verify(candidate.Bytes(), source.target, source.config.Network, source.config.Profile, time.Now())
	if err != nil {
		return nil, nil, err
	}
	stream := proposal.stream
	stream.mu.Lock()
	revision := stream.recipientRevision
	stream.mu.Unlock()
	value := descriptor.Publication.Delegation()
	identity := stream.owner.identity
	if descriptor.PublicationDigest != identity.Publication || value.Instance != identity.Instance || value.Generation != identity.InstanceGeneration || descriptor.Introduction.Revision < revision {
		return nil, nil, proposal.failIntegrity(nil)
	}
	if err := errors.Join(proposal.check(), descriptor.Current(time.Now())); err != nil {
		return nil, nil, err
	}
	intro, err := handle.ResolveSubmission(descriptor.Introduction.Node, source.config.Exclusions)
	if err != nil {
		return nil, nil, err
	}
	excluded := append(append([]route.Member(nil), source.config.Exclusions...), route.Member{NodeID: intro.NodeID, PublicKey: intro.PublicKey, FamilyID: intro.FamilyID})
	// At most the existing alternate is used; failed attempts cannot redraw or
	// expand the original context's retained Rendezvous set.
	rendezvous, until, err := handle.Rendezvous(source.config.Rendezvous, proposal.slot, excluded)
	if err != nil || source.config.WorkSafetyMaximum.After(until) {
		return nil, nil, errors.Join(errIntegrity, err)
	}
	if err := handle.CheckRendezvous(rendezvous, excluded); err != nil {
		return nil, nil, err
	}
	if source.config.PrepareRoute != nil {
		if err := source.config.PrepareRoute(proposal.ctx, handle, intro, rendezvous); err != nil {
			return nil, nil, fmt.Errorf("connection replacement Route preparation: %w", err)
		}
	}
	setupEnd := attachmentSetupDeadline(proposal.ctx, time.Now(), lookupEnd, source.config.WorkSafetyNotAfter, descriptor.Introduction.NotAfter)
	if !time.Now().Before(setupEnd) {
		return nil, nil, errRecoveryExhausted
	}
	request := capsule.Request{Network: identity.Network, Target: identity.Target, PublicationDigest: identity.Publication,
		Revision: descriptor.Introduction.Revision, RendezvousNode: rendezvous.NodeID, RendezvousDutyGeneration: rendezvous.RecordGeneration,
		ProfileDigest: identity.Profile, AttachmentGeneration: proposal.ticket.Generation(), Deadline: setupEnd,
		ConnectionNonce: identity.ConnectionNonce, InitiatorBinding: identity.InitiatorBinding,
		WorkSafetyNotAfter: identity.WorkSafetyNotAfter, WorkSafetyMaximum: identity.WorkSafetyMaximum, NoNewRecoveryAfter: identity.NoNewRecoveryAfter}
	defer clear(request.JoinSecret[:])
	if _, err := rand.Read(request.JoinSecret[:]); err != nil {
		return nil, nil, err
	}
	if _, err := rand.Read(request.HandshakeContext[:]); err != nil {
		return nil, nil, err
	}
	digest, err := request.Digest()
	if err != nil {
		return nil, nil, err
	}
	derived, err := deriveRecipientGeneration(descriptor.Publication, identity.Profile, request, digest, source.config.WorkSafetyMaximum, time.Now(), proposal.ticket.Generation())
	if err != nil || derived.Identity != identity || derived.Logical != stream.owner.logical {
		return nil, nil, errors.Join(errIntegrity, err)
	}
	binding := &InitiatorBinding{operation: source.config.Operation, setup: proposal.ctx, identity: derived.Identity, logical: derived.Logical,
		attachment: derived.Attachment, localContext: source.localContext, expires: setupEnd, revision: request.Revision,
		recipientEnd: descriptor.Introduction.NotAfter, proof: descriptor.Publication, source: source,
		joinSecretDigest: sha256.Sum256(request.JoinSecret[:]), joinContext: request.HandshakeContext}
	binding.self = binding
	header := capsule.Header{Slot: descriptor.Introduction.Slot, Revision: request.Revision, Expiry: setupEnd}
	if _, err := rand.Read(header.DeliveryNonce[:]); err != nil {
		return nil, nil, err
	}
	envelope, _, err := capsule.Seal(header, descriptor.Introduction.RecipientKey, request)
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true)); err != nil {
		return nil, nil, err
	}
	setup, cancel := context.WithDeadline(proposal.ctx, setupEnd)
	defer cancel()
	var physical *join.Joined
	err = runInitiatorSetupFlights(setup, acquisition.Close, func(ctx context.Context) error {
		if err := introduction.SubmitScoped(ctx, handle, intro, envelope, source.config.Exclusions); err != nil {
			return fmt.Errorf("connection replacement Introduction submission: %w", err)
		}
		return nil
	}, func() error {
		var err error
		physical, err = acquisition.Join(stream.owner.context(), join.Config{Duty: rendezvous,
			Secret: request.JoinSecret, Context: request.HandshakeContext, Deadline: source.config.WorkSafetyNotAfter, SetupDeadline: setupEnd})
		if err != nil {
			return fmt.Errorf("connection replacement JOIN: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if err := errors.Join(proposal.check(), binding.check(true), descriptor.Current(time.Now()), setup.Err()); err != nil {
		return nil, nil, err
	}
	return binding, physical, nil
}

// The exact physical handle stays within this permitted Route import owner.
type initiatorRouteAcquisition struct{ handle *prefix.SourceHandle }

func (route *initiatorRouteAcquisition) Commit(action func(func() error) error) error {
	if route == nil || route.handle == nil {
		return errIntegrity
	}
	return route.handle.Commit(action)
}

func (route *initiatorRouteAcquisition) close() {
	if route != nil {
		route.handle.Close()
	}
}

package connection

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// One finite original-key proposal owns all new physical resources until its
// exact generation commits. Neither setup cancellation nor rejected cleanup
// can reach an accepted Attachment or another Application operation.
type initiatorReplacement struct {
	stream                     *Stream
	ticket                     *recoveryProposal
	previous                   *attachmentRecords
	ctx                        context.Context
	cancel                     context.CancelFunc
	slot                       uint8
	acquisition                *join.JoinAcquisition
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
			if stream.recovery == nil || stream.recovery.stopped || stream.recovery.proposals >= recoveryProposalLimit || !time.Now().Before(stream.recovery.deadline) {
				return errors.Join(errRecoveryExhausted, last)
			}
			return nil
		})
		if err != nil {
			stream.mu.Lock()
			if !stream.closing && stream.ctx.Err() == nil && stream.records == records {
				stream.failLocked(errors.Join(last, err))
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
		ticket, err := stream.recovery.begin(records.generation, time.Now())
		if err != nil {
			return err
		}
		ctx, cancel := context.WithDeadline(stream.ctx, ticket.deadline)
		proposal = &initiatorReplacement{stream: stream, ticket: ticket, previous: records, ctx: ctx, cancel: cancel,
			interruptionDone: make(chan struct{}), prepared: make(chan struct{})}
		if stream.recovery.proposals > 1 {
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
	attachment.connection = tls.Client(initiatorTLSBorrow{attachment: attachment}, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		InsecureSkipVerify: true, // Exact independently verified Instance only.
		VerifyConnection: func(state tls.ConnectionState) error {
			return verifyInstanceCertificate(state, binding.identity.Instance)
		},
	})
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
	sendBase, sendEnd, receiveNext := stream.send.base, stream.send.end, stream.receive.next
	stream.mu.Unlock()
	local, err := wire.NewContinuity(key, wire.RoleInitiator, ticket.generation, sendBase, sendEnd, receiveNext, stream.owner.logical, exporter)
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
	receipt, err := wire.VerifyReplacement(evidence, key, *record.Continuity, wire.RolePublisher, ticket.generation, stream.owner.logical)
	if err != nil {
		return nil, proposal.failIntegrity(err)
	}
	peer, err := receipt.Consume(evidence, wire.RolePublisher, ticket.generation, stream.owner.logical, wire.ContinuityCommitment(key))
	if err != nil {
		return nil, proposal.failIntegrity(err)
	}
	stream.mu.Lock()
	err = errors.Join(stream.send.checkReplacement(ticket.generation, peer.ReceiveNext), stream.receive.checkReplacement(ticket.generation, peer.SendBase, peer.SendEnd))
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
		if proposal.closed || stream.initiatorProposal != proposal || stream.recovery == nil || !stream.recovery.owns(proposal.ticket) || !time.Now().Before(proposal.ticket.deadline) {
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
		proposal.closeErr = stream.recovery.reject(proposal.ticket, errors.Join(proposal.failure, proposal.ctx.Err(), proposal.interruptErr, retirement, physical))
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

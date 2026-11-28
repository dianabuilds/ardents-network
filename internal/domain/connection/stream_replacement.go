package connection

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
)

// Stream.mu admits one original interruption/join and clock for both physical
// actors, including simultaneous losses and peer proposals before local loss.
func (stream *Stream) startRecoveryLocked(records *attachmentRecords, cause error) error {
	if stream.records != records || records.generation != stream.transfer.Generation() || (stream.publisherRoute == nil && stream.initiatorRoute == nil) {
		return errIntegrity
	}
	if err := stream.checkLocked(); err != nil {
		return err
	}
	if records.retiring {
		return nil
	}
	if stream.recovery == nil {
		var err error
		stream.recovery, err = transfer.NewRecoveryEpisode(records.generation, stream.lastProgress, time.Unix(stream.owner.identity.NoNewRecoveryAfter, 0))
		if err != nil {
			return err
		}
	}
	if err := stream.recovery.Start(records.generation, time.Now()); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "qualification recovery-start role=%d ms=%d eof=%t short=%t\n", stream.owner.role, time.Now().UnixMilli(), errors.Is(cause, io.EOF), errors.Is(cause, io.ErrUnexpectedEOF))
	records.recoveryCause = cause
	records.retiring = true
	deadline := stream.recovery.Deadline()
	stream.recoveryUsers.Add(1)
	stream.signalLocked()
	go stream.joinFailedAttachment(records, deadline)
	if stream.initiatorRoute != nil {
		stream.recoveryUsers.Add(1)
		go stream.recoverInitiator(records)
	}
	return nil
}

func (stream *Stream) joinFailedAttachment(records *attachmentRecords, deadline time.Time) {
	defer stream.recoveryUsers.Done()
	go func() {
		var work, physical error
		if records.physical.initiator != nil {
			work = records.physical.initiator.closeFailed()
			physical = records.physical.initiator.cleanupErr
		} else {
			work = records.physical.service.CloseFailed()
			var joined bool
			physical, joined = records.physical.service.CleanupResult()
			if !joined {
				physical = errors.Join(physical, errIntegrity)
			}
		}
		work = errors.Join(work, records.join())
		stream.owner.operation.RetainCleanup(physical)
		stream.mu.Lock()
		if stream.recoveryLoss == nil {
			stream.recoveryLoss = work
		}
		stream.recoveryCleanup = errors.Join(stream.recoveryCleanup, physical)
		close(records.retired)
		stream.mu.Unlock()
	}()
	// A bound expires independently of a blocked physical join. Deny logical
	// effects on time, but retain the actor until that exact join really returns.
	defer func() { <-records.retired }()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-stream.ctx.Done():
	case <-records.replaced:
	case <-timer.C:
		stream.mu.Lock()
		if stream.records == records && !stream.closing {
			stream.failLocked(recoveryFailure(records.recoveryCause, errRecoveryExhausted))
		}
		stream.mu.Unlock()
	}
}

func (proposal *initiatorReplacement) commit() (result error) {
	<-proposal.prepared
	defer func() {
		if result != nil {
			result = errors.Join(result, proposal.close())
		}
	}()
	proposal.joinSetup()
	if err := errors.Join(proposal.check(), proposal.attachment.binding.check(true), proposal.attachment.check()); err != nil {
		return err
	}
	stream := proposal.stream
	source := stream.initiatorRoute
	return proposal.sourceHandle.Commit(func(checkOriginal func() error) error {
		stream.mu.Lock()
		defer stream.mu.Unlock()
		source.mu.Lock()
		defer source.mu.Unlock()
		if proposal.closed || proposal.transferred || proposal.failure != nil || stream.initiatorProposal != proposal ||
			stream.records != proposal.previous || stream.recovery == nil || !stream.recovery.Owns(proposal.ticket) ||
			stream.closing || stream.ctx.Err() != nil || source.owner != stream.owner || source.stream != stream ||
			proposal.attachment.binding.revision < stream.recipientRevision || !stream.transfer.CanReplace() {
			return errIntegrity
		}
		select {
		case <-proposal.previous.retired:
		default:
			return errIntegrity
		}
		now := time.Now()
		if !now.Before(proposal.ticket.Deadline()) || !now.Before(proposal.attachment.binding.expires) || !now.Before(proposal.attachment.binding.recipientEnd) {
			return errRecoveryExhausted
		}
		if err := errors.Join(checkOriginal(), source.localLocked(), proposal.ctx.Err(), stream.owner.context().Err()); err != nil {
			return err
		}
		if err := stream.owner.checkLogicalCommit(); err != nil {
			return err
		}
		proposal.attachment.mu.Lock()
		attachmentErr := proposal.attachment.checkLocked()
		proposal.attachment.mu.Unlock()
		if attachmentErr != nil {
			return attachmentErr
		}
		if _, _, err := stream.contract.Limits(stream.owner.operation); err != nil {
			return err
		}
		nextTransfer, err := stream.transfer.PrepareReplacement(proposal.ticket.Generation(), proposal.peer)
		if err != nil {
			return err
		}
		if err := stream.recovery.Commit(proposal.ticket, now); err != nil {
			nextTransfer.Discard()
			return err
		}
		nextTransfer.Commit()
		stream.recipientRevision = proposal.attachment.binding.revision
		// Logical provenance stays with the immutable original authentication;
		// the new record owner retains this exact physical child through Close.
		next := &attachmentRecords{generation: proposal.ticket.Generation(),
			physical:   attachmentIO{owner: stream.owner, initiator: proposal.attachment, context: proposal.attachment.binding.attachment},
			readerDone: make(chan struct{}), writerDone: make(chan struct{}), retired: make(chan struct{}), replaced: make(chan struct{})}
		stream.records = next
		stream.initiatorProposal = nil
		proposal.closed, proposal.transferred = true, true
		close(proposal.previous.replaced)
		proposal.cancel()
		stream.signalLocked()
		go stream.readRecords(next)
		go stream.writeRecords(next)
		return nil
	})
}

// Commit transfers only this completed original-key proposal, after both old
// actors and setup callbacks have joined. It atomically advances all logical
// ledgers, preserving accepted byte counters and starting the replay writer
// immediately; it never opens another Application request or grants authority.
func (proposal *PublisherReplacement) Commit() (result error) {
	if proposal == nil || proposal.self != proposal || proposal.stream == nil || proposal.prepared == nil {
		return errIntegrity
	}
	proposal.commitMu.Lock()
	defer proposal.commitMu.Unlock()
	<-proposal.prepared
	stream := proposal.stream
	stream.mu.Lock()
	accepted := proposal.transferred
	stream.mu.Unlock()
	if accepted {
		return nil // Retained original transfer, never a second permission.
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, proposal.Close())
		}
	}()
	proposal.joinSetup()
	// Observe the original Instance/native authority before taking the
	// logical state lock; the local gate below rechecks seals and identity.
	if err := stream.owner.checkLogical(); err != nil {
		return err
	}
	stream.mu.Lock()
	if err := proposal.checkCommitStateLocked(); err != nil {
		stream.mu.Unlock()
		return err
	}
	stream.mu.Unlock()
	if _, _, err := proposal.opening.Request(proposal.ctx); err != nil {
		return err
	}
	if err := proposal.service.TransferReplacement(proposal.ctx); err != nil {
		return err
	}
	if err := stream.publisherRoute.check(stream.owner); err != nil {
		return err
	}
	if err := proposal.service.CheckIdentity(stream.owner.identity, stream.owner.logical, proposal.attachment); err != nil {
		return err
	}
	return proposal.route.commit(func(checkOriginal func() error) error {
		stream.mu.Lock()
		defer stream.mu.Unlock()
		if err := errors.Join(checkOriginal(), proposal.checkCommitStateLocked(), proposal.service.CheckReplacementCommit(proposal.ctx)); err != nil {
			return err
		}
		// Prepare all next state without trimming the live replay allocation.
		// None of the three ledger transitions can leave a partial generation.
		nextTransfer, err := stream.transfer.PrepareReplacement(proposal.ticket.Generation(), proposal.peer)
		if err != nil {
			return err
		}
		if err := stream.recovery.Commit(proposal.ticket, time.Now()); err != nil {
			nextTransfer.Discard()
			return err
		}
		nextTransfer.Commit()
		stream.recipientRevision = proposal.revision
		next := &attachmentRecords{generation: proposal.ticket.Generation(),
			physical:   attachmentIO{owner: stream.owner, service: proposal.service, context: proposal.attachment},
			readerDone: make(chan struct{}), writerDone: make(chan struct{}), retired: make(chan struct{}), replaced: make(chan struct{})}
		stream.records = next
		stream.publisherProposal = nil
		proposal.transferred, proposal.closed = true, true
		close(proposal.previous.replaced)
		proposal.cancel() // Joined callbacks no longer interrupt the accepted TLS.
		stream.signalLocked()
		go stream.readRecords(next)
		go stream.writeRecords(next)
		return nil
	})
}

// Native authority is reobserved above before entering Source generation locks.
// This final transition checks only local seals, contexts, time and byte state.
func (proposal *PublisherReplacement) checkCommitStateLocked() error {
	if proposal.caller == nil {
		return errIntegrity
	}
	// Joining the setup callback detaches future interruption of an accepted
	// child. It does not detach caller authority before the actual transfer.
	if err := proposal.caller.Err(); err != nil {
		return err
	}
	stream := proposal.stream
	if proposal.closed || proposal.transferred || proposal.failure != nil || proposal.service == nil || proposal.previous == nil ||
		proposal.revision < stream.recipientRevision || proposal.revision == 0 ||
		stream.publisherProposal != proposal || stream.records != proposal.previous || stream.recovery == nil || !stream.recovery.Owns(proposal.ticket) ||
		proposal.opening == nil || !stream.transfer.CanReplace() || !time.Now().Before(proposal.ticket.Deadline()) {
		return errIntegrity
	}
	select {
	case <-proposal.previous.retired:
	default:
		return errIntegrity
	}
	if stream.closing || stream.failure != nil || stream.ctx.Err() != nil {
		return errors.Join(errIntegrity, stream.failure, stream.ctx.Err())
	}
	if err := stream.owner.checkLogicalCommit(); err != nil {
		return err
	}
	if _, _, err := stream.contract.Limits(stream.owner.operation); err != nil {
		return err
	}
	return errors.Join(proposal.ctx.Err(), stream.owner.context().Err(),
		stream.transfer.CheckReplacement(proposal.ticket.Generation(), proposal.peer))
}

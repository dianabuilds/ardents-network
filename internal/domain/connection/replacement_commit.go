package connection

import (
	"errors"
	"time"
)

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
			stream.records != proposal.previous || stream.recovery == nil || !stream.recovery.owns(proposal.ticket) ||
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
		if !now.Before(proposal.ticket.deadline) || !now.Before(proposal.attachment.binding.expires) || !now.Before(proposal.attachment.binding.recipientEnd) {
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
		nextTransfer, err := stream.transfer.PrepareReplacement(proposal.ticket.generation, proposal.peer)
		if err != nil {
			return err
		}
		if err := stream.recovery.commit(proposal.ticket, now); err != nil {
			nextTransfer.Discard()
			return err
		}
		nextTransfer.Commit()
		stream.recipientRevision = proposal.attachment.binding.revision
		// Logical provenance stays with the immutable original authentication;
		// the new record owner retains this exact physical child through Close.
		next := &attachmentRecords{generation: proposal.ticket.generation,
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
		nextTransfer, err := stream.transfer.PrepareReplacement(proposal.ticket.generation, proposal.peer)
		if err != nil {
			return err
		}
		if err := stream.recovery.commit(proposal.ticket, time.Now()); err != nil {
			nextTransfer.Discard()
			return err
		}
		nextTransfer.Commit()
		stream.recipientRevision = proposal.revision
		next := &attachmentRecords{generation: proposal.ticket.generation,
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
		stream.publisherProposal != proposal || stream.records != proposal.previous || stream.recovery == nil || !stream.recovery.owns(proposal.ticket) ||
		proposal.opening == nil || !stream.transfer.CanReplace() || !time.Now().Before(proposal.ticket.deadline) {
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
		stream.transfer.CheckReplacement(proposal.ticket.generation, proposal.peer))
}

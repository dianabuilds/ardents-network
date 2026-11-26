package connection

import (
	"errors"
	"time"
)

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
		send, receive, terminal := *stream.send, *stream.receive, *stream.terminal
		send.data = append([]byte(nil), send.data...)
		if err := errors.Join(send.replace(proposal.ticket.generation, proposal.peer.ReceiveNext),
			receive.replace(proposal.ticket.generation, proposal.peer.SendBase, proposal.peer.SendEnd),
			terminal.replace(proposal.ticket.generation)); err != nil {
			clear(send.data)
			return err
		}
		if err := stream.recovery.commit(proposal.ticket, time.Now()); err != nil {
			clear(send.data)
			return err
		}
		clear(stream.send.data)
		stream.send, stream.receive, stream.terminal = &send, &receive, &terminal
		stream.recipientRevision = proposal.revision
		stream.ackWritten = stream.ackSerial
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
		proposal.opening == nil || stream.send.active != nil || stream.terminal.active != nil ||
		stream.send.generation != stream.receive.generation || stream.send.generation != stream.terminal.generation || !time.Now().Before(proposal.ticket.deadline) {
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
		stream.send.checkReplacement(proposal.ticket.generation, proposal.peer.ReceiveNext),
		stream.receive.checkReplacement(proposal.ticket.generation, proposal.peer.SendBase, proposal.peer.SendEnd))
}

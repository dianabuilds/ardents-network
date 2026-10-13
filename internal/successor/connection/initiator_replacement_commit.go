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
			proposal.attachment.binding.revision < stream.recipientRevision || stream.send.active != nil || stream.terminal.active != nil {
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
		send, receive, terminal := *stream.send, *stream.receive, *stream.terminal
		send.data = append([]byte(nil), send.data...)
		if err := errors.Join(send.replace(proposal.ticket.generation, proposal.peer.ReceiveNext),
			receive.replace(proposal.ticket.generation, proposal.peer.SendBase, proposal.peer.SendEnd), terminal.replace(proposal.ticket.generation)); err != nil {
			clear(send.data)
			return err
		}
		if err := stream.recovery.commit(proposal.ticket, now); err != nil {
			clear(send.data)
			return err
		}
		clear(stream.send.data)
		stream.send, stream.receive, stream.terminal = &send, &receive, &terminal
		stream.recipientRevision = proposal.attachment.binding.revision
		stream.ackWritten = stream.ackSerial
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

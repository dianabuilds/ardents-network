package connection

import "errors"

// replaySettledTerminal preserves a completed local half-close when another
// worker recovered its Attachment after the sender had already returned. A
// successful local write does not prove the peer received that Terminal.
func (stream *Stream) replaySettledTerminal() error {
	for {
		stream.mu.Lock()
		replay := stream.localTerminal && stream.terminalSettled &&
			(stream.terminalAcknowledgedGeneration == 0 || stream.postClose) &&
			stream.terminal == nil && stream.current != nil && stream.terminalGeneration != stream.current.generation
		stream.mu.Unlock()
		if !replay {
			return nil
		}
		if err := stream.flushAvailable(); err != nil {
			return err
		}
		if err := stream.ensureTerminal(); err != nil {
			return err
		}
	}
}

// startSettledTerminalReplay keeps the receive worker available to consume the
// peer's simultaneous terminal while this stream replays its own terminal on
// a recovered Attachment.
func (stream *Stream) startSettledTerminalReplay() {
	stream.mu.Lock()
	replay := stream.localTerminal && stream.terminalSettled &&
		(stream.terminalAcknowledgedGeneration == 0 || stream.postClose) &&
		stream.terminal == nil && stream.current != nil && stream.terminalGeneration != stream.current.generation && !stream.terminalReplaying
	if replay {
		stream.terminalReplaying = true
	}
	stream.mu.Unlock()
	if !replay {
		return
	}
	go func() {
		err := stream.replaySettledTerminal()
		stream.mu.Lock()
		stream.terminalReplaying = false
		stream.cond.Broadcast()
		stream.mu.Unlock()
		if err != nil {
			stream.fail(err)
		}
	}()
}

func (stream *Stream) ensureTerminal() error {
	attachment, err := stream.attachment()
	if err != nil {
		return err
	}
	stream.mu.Lock()
	if !stream.localTerminal || stream.terminal != nil || stream.terminalGeneration == attachment.generation {
		stream.mu.Unlock()
		return stream.terminal
	}
	offset := stream.sendEnd
	stream.terminalWriting = true
	stream.terminalWritingGeneration = attachment.generation
	stream.terminalOffset = offset
	stream.mu.Unlock()
	err = stream.writeRecord(attachment, StreamRecord{Terminal: &Terminal{AttachmentGeneration: attachment.generation, Offset: offset}})
	if err != nil {
		// Keep terminalWriting true until this worker has either acquired or
		// joined recovery. A read worker that closed the failed carrier waits
		// on that ownership boundary instead of racing a second proposal.
		recoverErr := stream.recoverAttachment(attachment)
		stream.mu.Lock()
		stream.terminalWriting = false
		stream.cond.Broadcast()
		stream.mu.Unlock()
		if recoverErr != nil {
			return errors.Join(errRecoveryTerminal, err, recoverErr)
		}
		return nil
	}
	stream.mu.Lock()
	stream.terminalWriting = false
	if stream.current == attachment {
		stream.terminalGeneration = attachment.generation
	}
	stream.cond.Broadcast()
	stream.mu.Unlock()
	return nil
}

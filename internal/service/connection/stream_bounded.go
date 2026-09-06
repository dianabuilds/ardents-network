package connection

import (
	"context"
	"errors"
	"io"
	"time"
)

// RunBounded carries one live bidirectional byte stream up to the declared
// directional limits. EOF from either local Application direction becomes the
// existing authenticated Terminal record at its exact logical offset; it is a
// normal half-close rather than an exact-workload failure.
func (stream *Stream) RunBounded(sendLimit, receiveLimit uint32) (Outcome, error) {
	stream.watchNameOrigin()
	stop := context.AfterFunc(stream.ctx, func() { stream.fail(stream.ctx.Err()) })
	var releaseSafety func()
	tail := false
	defer func() {
		if tail {
			return
		}
		stop()
		if releaseSafety != nil {
			releaseSafety()
		}
		close(stream.done)
		stream.close()
	}()
	if err := stream.establishInitialAttachment(); err != nil {
		stream.fail(err)
		return stream.outcome(), err
	}
	if stream.recovery.WorkSafetyNotAfter != 0 {
		remaining := time.Unix(stream.recovery.WorkSafetyNotAfter, 0).Sub(stream.authorizationTime())
		releaseTimer := acquireResource(stream.resources, "timer")
		safetyTimer := time.AfterFunc(remaining, func() { stream.fail(errWorkSafetyExpired) })
		releaseSafety = func() {
			safetyTimer.Stop()
			releaseTimer()
		}
	}
	sendResult := make(chan error, 1)
	receiveResult := make(chan error, 1)
	ackResult := make(chan error, 1)
	go func() { sendResult <- stream.sendApplicationBounded(uint64(sendLimit)) }()
	go func() {
		err := stream.receiveApplicationBounded(uint64(receiveLimit))
		if err != nil {
			stream.fail(err)
		}
		receiveResult <- err
	}()
	go func() { ackResult <- stream.sendBoundedAcknowledgements() }()
	var sendErr, receiveErr, acknowledgementErr error
	sendDone, receiveDone, acknowledgementDone := false, false, false
	for !sendDone || !acknowledgementDone {
		select {
		case sendErr = <-sendResult:
			sendDone = true
			if sendErr != nil {
				stream.fail(sendErr)
			}
		case receiveErr = <-receiveResult:
			receiveDone = true
			if receiveErr != nil {
				stream.fail(receiveErr)
			}
		case acknowledgementErr = <-ackResult:
			acknowledgementDone = true
			if acknowledgementErr != nil {
				stream.fail(acknowledgementErr)
			}
		}
	}
	err := errors.Join(sendErr, receiveErr, acknowledgementErr)
	if err != nil && !receiveDone {
		receiveErr = <-receiveResult
		receiveDone = true
		err = errors.Join(sendErr, receiveErr, acknowledgementErr)
	}
	outcome := stream.outcome()
	if err == nil && outcome.Acknowledged != outcome.Accepted {
		err = errors.New("bounded Application stream closed before its final bytes were acknowledged")
	}
	stream.mu.Lock()
	if err == nil {
		err = stream.terminal
	}
	stream.mu.Unlock()
	if err == nil {
		tailSafety := releaseSafety
		tailReceiver := receiveResult
		if receiveDone {
			tailReceiver = nil
		}
		tail = stream.startTerminalTail(func() {
			stop()
			if tailSafety != nil {
				tailSafety()
			}
		}, tailReceiver)
		if tail {
			releaseSafety = nil
		}
	}
	if !tail && !receiveDone {
		receiveErr = <-receiveResult
		err = errors.Join(err, receiveErr)
	}
	return outcome, err
}

func (stream *Stream) sendApplicationBounded(limit uint64) error {
	buffer := make([]byte, MaximumDataBytes)
	for {
		stream.mu.Lock()
		for stream.sendQueueBlockedLocked() && stream.terminal == nil {
			stream.cond.Wait()
		}
		if stream.terminal != nil {
			err := stream.terminal
			stream.mu.Unlock()
			return err
		}
		if stream.sendNext < stream.sendEnd {
			stream.mu.Unlock()
			if err := stream.flushAvailable(); err != nil {
				return err
			}
			continue
		}
		remaining, available := limit-stream.sendEnd, logicalQueueLimit-len(stream.sendData)
		if remaining == 0 {
			stream.mu.Unlock()
			read, err := stream.application.Read(buffer[:1])
			if read > 0 {
				return errors.New("Application input exceeded its directional byte bound")
			}
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
					return stream.finishBoundedSend()
				}
				return err
			}
			continue
		}
		want := len(buffer)
		if uint64(want) > remaining {
			want = int(remaining)
		}
		if want > available {
			want = available
		}
		stream.mu.Unlock()

		read, err := stream.application.Read(buffer[:want])
		if read > 0 {
			stream.mu.Lock()
			stream.sendData = append(stream.sendData, buffer[:read]...)
			stream.sendEnd += uint64(read)
			stream.lastProgress = time.Now()
			if uint32(len(stream.sendData)) > stream.queueMax {
				stream.queueMax = uint32(len(stream.sendData))
			}
			stream.mu.Unlock()
			if flushErr := stream.flushAvailable(); flushErr != nil {
				return flushErr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return stream.finishBoundedSend()
			}
			return err
		}
	}
}

func (stream *Stream) finishBoundedSend() error {
	stream.mu.Lock()
	stream.localTerminal = true
	stream.mu.Unlock()
	stream.signalAcknowledgement()
	for {
		if err := stream.ensureTerminal(); err != nil {
			return err
		}
		stream.mu.Lock()
		terminal := stream.terminal
		sent := stream.current != nil && stream.terminalGeneration == stream.current.generation
		stream.mu.Unlock()
		if terminal != nil {
			return terminal
		}
		if sent {
			stream.mu.Lock()
			stream.terminalSettled = true
			stream.mu.Unlock()
			// Recovery may have committed between the successful write and this
			// settlement mark. Recheck now so that the new Attachment receives
			// the durable Terminal obligation in that interleaving as well.
			stream.startSettledTerminalReplay()
			return nil
		}
	}
}

func (stream *Stream) receiveApplicationBounded(limit uint64) error {
	for {
		stream.mu.Lock()
		terminal := stream.terminal
		complete := stream.boundedReceiveCompleteLocked()
		stream.mu.Unlock()
		if terminal != nil {
			return terminal
		}
		if complete {
			return nil
		}
		attachment, err := stream.attachment()
		if err != nil {
			return err
		}
		record, err := ReadStream(attachment.carrier)
		if err != nil {
			stream.mu.Lock()
			writingTerminal := stream.terminalReplaying || stream.terminalWriting
			stream.mu.Unlock()
			if writingTerminal && stream.opener != nil {
				// Recovery owns the failed carrier. Closing it here releases a
				// serialized write so its worker can perform the one coordinated
				// replacement instead of leaving both workers blocked.
				attachment.closeCarrier()
			}
			stream.mu.Lock()
			for writingTerminal && (stream.terminalReplaying || stream.terminalWriting) && stream.terminal == nil {
				stream.cond.Wait()
			}
			terminal = stream.terminal
			complete = stream.boundedReceiveCompleteLocked()
			stream.mu.Unlock()
			if terminal != nil {
				return terminal
			}
			if complete && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe)) {
				return nil
			}
			if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
				return errors.Join(errRecoveryTerminal, err, recoverErr)
			}
			continue
		}
		var generation, offset uint64
		switch {
		case record.Data != nil:
			generation, offset = record.Data.AttachmentGeneration, record.Data.Offset
		case record.Acknowledgement != nil:
			generation, offset = record.Acknowledgement.AttachmentGeneration, record.Acknowledgement.Offset
		case record.Terminal != nil:
			generation, offset = record.Terminal.AttachmentGeneration, record.Terminal.Offset
		default:
			return ErrActiveViolation
		}
		if generation != attachment.generation {
			return errors.Join(ErrActiveViolation, errors.New("record names a stale Service Connection Attachment"))
		}
		switch {
		case record.Acknowledgement != nil:
			stream.mu.Lock()
			if record.Acknowledgement.TerminalConfirmation {
				receiptSent := stream.terminalAckSent &&
					record.Acknowledgement.AttachmentGeneration == stream.terminalAckGeneration &&
					offset == stream.terminalAckOffset
				receiptWriting := stream.terminalAckWriting &&
					record.Acknowledgement.AttachmentGeneration == stream.terminalAckWritingGeneration &&
					offset == stream.terminalAckWritingOffset
				valid := stream.remoteTerminal && stream.terminalAckPending && (receiptSent || receiptWriting)
				if valid {
					stream.terminalAckConfirmedGeneration = record.Acknowledgement.AttachmentGeneration
				}
				stream.mu.Unlock()
				if !valid {
					return errors.Join(ErrActiveViolation, errors.New("Terminal receipt confirmation does not match a sent receipt"))
				}
				stream.signalAcknowledgement()
				continue
			}
			terminalReceipt := record.Acknowledgement.Terminal && stream.localTerminal && offset == stream.terminalOffset &&
				(record.Acknowledgement.AttachmentGeneration == stream.terminalGeneration ||
					(stream.terminalWriting && record.Acknowledgement.AttachmentGeneration == stream.terminalWritingGeneration))
			if record.Acknowledgement.Terminal && !terminalReceipt {
				stream.mu.Unlock()
				return errors.Join(ErrActiveViolation, errors.New("Terminal receipt does not match a local Terminal"))
			}
			err = stream.acknowledgeLocked(offset)
			if err == nil && terminalReceipt {
				stream.terminalAcknowledgedGeneration = record.Acknowledgement.AttachmentGeneration
				if stream.opener != nil {
					stream.terminalConfirmationPending = true
					stream.terminalConfirmationSent = false
					stream.terminalConfirmationGeneration = record.Acknowledgement.AttachmentGeneration
					stream.terminalConfirmationOffset = offset
				}
			}
			stream.mu.Unlock()
			stream.signalAcknowledgement()
		case record.Data != nil:
			err = stream.acceptData(record.Data, limit)
		case record.Terminal != nil:
			stream.mu.Lock()
			valid := offset == stream.recvNext
			firstTerminal := valid && !stream.remoteTerminal
			closeApplication := firstTerminal && stream.closeApplicationOnRemoteTerminal
			if valid {
				stream.remoteTerminal = true
				if offset > stream.ackPending {
					stream.ackPending = offset
				}
				if stream.opener != nil && stream.terminalAckGeneration != attachment.generation {
					stream.terminalAckPending = true
					stream.terminalAckSent = false
					stream.terminalAckPendingGeneration = attachment.generation
					stream.terminalAckOffset = offset
				}
			}
			stream.mu.Unlock()
			if !valid {
				return errors.Join(ErrActiveViolation, errors.New("Terminal does not match the received logical offset"))
			}
			if firstTerminal {
				if closeApplication {
					err = stream.application.Close()
				} else {
					err = stream.application.CloseInput()
				}
				if err != nil {
					return err
				}
			}
			stream.signalAcknowledgement()
			continue
		}
		if err != nil {
			return err
		}
	}
}

func (stream *Stream) sendBoundedAcknowledgements() error {
	for {
		stream.mu.Lock()
		if stream.terminal != nil {
			err := stream.terminal
			stream.mu.Unlock()
			return err
		}
		complete := stream.boundedAcknowledgementCompleteLocked()
		stream.mu.Unlock()
		if complete {
			return nil
		}
		select {
		case <-stream.ackSignal:
		case <-stream.ctx.Done():
			return stream.ctx.Err()
		}
		for {
			stream.mu.Lock()
			offset, already := stream.ackPending, stream.ackSent
			stream.mu.Unlock()
			attachment, err := stream.attachment()
			if err != nil {
				return err
			}
			stream.mu.Lock()
			terminal := stream.terminalAckPending && !stream.terminalAckSent &&
				stream.terminalAckPendingGeneration == attachment.generation
			confirmation := !terminal && stream.terminalConfirmationPending && !stream.terminalConfirmationSent &&
				stream.terminalConfirmationGeneration == attachment.generation
			if confirmation {
				offset = stream.terminalConfirmationOffset
			}
			if terminal {
				stream.terminalAckWriting = true
				stream.terminalAckWritingGeneration = attachment.generation
				stream.terminalAckWritingOffset = offset
			}
			stream.mu.Unlock()
			if offset <= already && !terminal && !confirmation {
				break
			}
			if err := stream.writeRecord(attachment, StreamRecord{Acknowledgement: &Acknowledgement{
				AttachmentGeneration: attachment.generation, Offset: offset, Terminal: terminal || confirmation,
				TerminalConfirmation: confirmation,
			}}); err != nil {
				if terminal {
					stream.mu.Lock()
					if stream.terminalAckWritingGeneration == attachment.generation && stream.terminalAckWritingOffset == offset {
						stream.terminalAckWriting = false
					}
					stream.mu.Unlock()
				}
				if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
					return errors.Join(errRecoveryTerminal, err, recoverErr)
				}
				continue
			}
			stream.mu.Lock()
			if !confirmation && offset > stream.ackSent {
				stream.ackSent = offset
			}
			if terminal && stream.terminalAckWritingGeneration == attachment.generation &&
				stream.terminalAckWritingOffset == offset {
				stream.terminalAckWriting = false
			}
			if terminal && stream.current == attachment {
				stream.terminalAckSent = true
				stream.terminalAckGeneration = attachment.generation
			}
			if confirmation && stream.current == attachment {
				stream.terminalConfirmationSent = true
			}
			stream.mu.Unlock()
		}
	}
}

func (stream *Stream) boundedReceiveCompleteLocked() bool {
	return !stream.postClose && stream.remoteTerminal && stream.localTerminal && stream.sendBase == stream.sendEnd &&
		(stream.opener == nil || stream.terminalAcknowledgedGeneration != 0) &&
		(!stream.terminalAckPending || (stream.terminalAckSent &&
			stream.terminalAckConfirmedGeneration == stream.terminalAckGeneration))
}

func (stream *Stream) boundedAcknowledgementCompleteLocked() bool {
	return stream.boundedReceiveCompleteLocked() && stream.ackSent >= stream.ackPending &&
		(!stream.terminalConfirmationPending || stream.terminalConfirmationSent)
}

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
	defer close(stream.done)
	stream.watchNameOrigin()
	stop := context.AfterFunc(stream.ctx, func() { stream.fail(stream.ctx.Err()) })
	defer stop()
	if err := stream.establishInitialAttachment(); err != nil {
		stream.fail(err)
		return stream.outcome(), err
	}
	if stream.recovery.WorkSafetyNotAfter != 0 {
		remaining := time.Unix(stream.recovery.WorkSafetyNotAfter, 0).Sub(stream.authorizationTime())
		releaseTimer := acquireResource(stream.resources, "timer")
		safetyTimer := time.AfterFunc(remaining, func() { stream.fail(errWorkSafetyExpired) })
		defer func() {
			safetyTimer.Stop()
			releaseTimer()
		}()
	}
	defer stream.close()
	dataResults := make(chan error, 2)
	ackResult := make(chan error, 1)
	go func() { dataResults <- stream.sendApplicationBounded(uint64(sendLimit)) }()
	go func() { dataResults <- stream.receiveApplicationBounded(uint64(receiveLimit)) }()
	go func() { ackResult <- stream.sendBoundedAcknowledgements() }()
	first, second := <-dataResults, <-dataResults
	dataErr := errors.Join(first, second)
	if dataErr != nil {
		stream.fail(dataErr)
	}
	err := errors.Join(dataErr, <-ackResult)
	stream.mu.Lock()
	if err == nil {
		err = stream.terminal
	}
	stream.mu.Unlock()
	return stream.outcome(), err
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
		if stream.sendEnd >= limit {
			stream.mu.Unlock()
			return stream.finishBoundedSend()
		}
		remaining, available := limit-stream.sendEnd, logicalQueueLimit-len(stream.sendData)
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
	for {
		if err := stream.ensureTerminal(); err != nil {
			return err
		}
		stream.mu.Lock()
		if stream.terminal != nil {
			err := stream.terminal
			stream.mu.Unlock()
			return err
		}
		if stream.sendBase == stream.sendEnd {
			stream.mu.Unlock()
			return nil
		}
		stream.cond.Wait()
		stream.mu.Unlock()
	}
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
	stream.mu.Unlock()
	if err := stream.writeRecord(attachment, StreamRecord{Terminal: &Terminal{AttachmentGeneration: attachment.generation, Offset: offset}}); err != nil {
		if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
			return errors.Join(errRecoveryTerminal, err, recoverErr)
		}
		return nil
	}
	stream.mu.Lock()
	if stream.current == attachment {
		stream.terminalGeneration = attachment.generation
	}
	stream.mu.Unlock()
	return nil
}

func (stream *Stream) receiveApplicationBounded(limit uint64) error {
	for {
		stream.mu.Lock()
		terminal := stream.terminal
		complete := stream.remoteTerminal && stream.localTerminal && stream.sendBase == stream.sendEnd
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
			complete = stream.remoteTerminal && stream.localTerminal && stream.sendBase == stream.sendEnd
			stream.mu.Unlock()
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
			return ErrActiveViolation
		}
		switch {
		case record.Acknowledgement != nil:
			stream.mu.Lock()
			err = stream.acknowledgeLocked(offset)
			stream.mu.Unlock()
		case record.Data != nil:
			err = stream.acceptData(record.Data, limit)
		case record.Terminal != nil:
			stream.mu.Lock()
			valid := offset == stream.recvNext
			if valid {
				stream.remoteTerminal = true
			}
			stream.mu.Unlock()
			if !valid {
				return ErrActiveViolation
			}
			stream.queueAcknowledgement(offset)
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
		complete := stream.remoteTerminal && stream.ackSent >= stream.ackPending
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
			if offset <= already {
				break
			}
			attachment, err := stream.attachment()
			if err != nil {
				return err
			}
			if err := stream.writeRecord(attachment, StreamRecord{Acknowledgement: &Acknowledgement{AttachmentGeneration: attachment.generation, Offset: offset}}); err != nil {
				if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
					return errors.Join(errRecoveryTerminal, err, recoverErr)
				}
				continue
			}
			stream.mu.Lock()
			if offset > stream.ackSent {
				stream.ackSent = offset
			}
			stream.mu.Unlock()
		}
	}
}

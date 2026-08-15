package serviceconn

import (
	"errors"
	"io"
)

func (stream *recoveryStream) sendApplication(limit uint64) error {
	buffer := make([]byte, maximumFrameData)
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
			for stream.sendBase < limit && stream.terminal == nil {
				stream.cond.Wait()
			}
			err := stream.terminal
			stream.mu.Unlock()
			return err
		}
		remaining := limit - stream.sendEnd
		available := logicalQueueLimit - len(stream.sendData)
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
			if uint32(len(stream.sendData)) > stream.queueMax {
				stream.queueMax = uint32(len(stream.sendData))
			}
			stream.mu.Unlock()
			if flushErr := stream.flushAvailable(); flushErr != nil {
				return flushErr
			}
		}
		if err != nil {
			stream.mu.Lock()
			complete := stream.sendEnd >= limit
			stream.mu.Unlock()
			if !complete {
				stream.sendTerminal()
				return errors.Join(err, errors.New("application stream ended before the declared byte count"))
			}
		}
	}
}

func (stream *recoveryStream) sendQueueBlockedLocked() bool {
	return len(stream.sendData) >= logicalQueueLimit && stream.sendNext >= stream.sendEnd
}

func (stream *recoveryStream) sendTerminal() {
	attachment, err := stream.attachment()
	if err != nil {
		return
	}
	stream.mu.Lock()
	offset := stream.sendEnd
	stream.mu.Unlock()
	_ = stream.writeFrame(attachment, connectionFrame{kind: terminalFrameType,
		generation: attachment.generation, offset: offset})
}

func (stream *recoveryStream) flushAvailable() error {
	for {
		stream.mu.Lock()
		if stream.terminal != nil {
			err := stream.terminal
			stream.mu.Unlock()
			return err
		}
		if stream.sendNext >= stream.sendEnd {
			stream.mu.Unlock()
			return nil
		}
		offset := stream.sendNext
		start := offset - stream.sendBase
		length := stream.sendEnd - offset
		if length > maximumFrameData {
			length = maximumFrameData
		}
		payload := append([]byte(nil), stream.sendData[start:start+length]...)
		stream.mu.Unlock()

		attachment, err := stream.attachment()
		if err != nil {
			return err
		}
		frame := connectionFrame{kind: dataFrameType, generation: attachment.generation, offset: offset, data: payload}
		if err := stream.writeFrame(attachment, frame); err != nil {
			if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
				return errors.Join(errRecoveryTerminal, err, recoverErr)
			}
			continue
		}
		stream.mu.Lock()
		if offset == stream.sendNext {
			stream.sendNext += uint64(len(payload))
		}
		stream.mu.Unlock()
	}
}

func (stream *recoveryStream) writeFrame(attachment *securedAttachment, frame connectionFrame) error {
	stream.writerMu.Lock()
	defer stream.writerMu.Unlock()
	stream.mu.Lock()
	current := stream.current == attachment && !stream.recovering && stream.terminal == nil
	stream.mu.Unlock()
	if !current {
		return io.ErrClosedPipe
	}
	return writeConnectionFrame(attachment.connection, frame)
}

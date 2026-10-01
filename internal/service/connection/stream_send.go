package connection

import (
	"errors"
	"io"
)

func (stream *Stream) sendQueueBlockedLocked() bool {
	return len(stream.sendData) >= logicalQueueLimit && stream.sendNext >= stream.sendEnd
}

func (stream *Stream) flushAvailable() error {
	stream.flushMu.Lock()
	defer stream.flushMu.Unlock()
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
		start, length := offset-stream.sendBase, stream.sendEnd-offset
		if length > MaximumDataBytes {
			length = MaximumDataBytes
		}
		payload := append([]byte(nil), stream.sendData[start:start+length]...)
		stream.mu.Unlock()

		attachment, err := stream.attachment()
		if err != nil {
			return err
		}
		if err := stream.writeRecord(attachment, StreamRecord{Data: &Data{
			AttachmentGeneration: attachment.generation, Offset: offset, Payload: payload}}); err != nil {
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

func (stream *Stream) writeRecord(attachment *Attachment, record StreamRecord) error {
	stream.writerMu.Lock()
	defer stream.writerMu.Unlock()
	stream.mu.Lock()
	current := stream.current == attachment && !stream.recovering && stream.terminal == nil
	pendingTerminalData := current && record.Terminal != nil && stream.sendNext < stream.sendEnd
	stream.mu.Unlock()
	if pendingTerminalData {
		return errTerminalDataPending
	}
	if !current {
		return io.ErrClosedPipe
	}
	return Write(attachment.carrier, Record{Data: record.Data, Acknowledgement: record.Acknowledgement, Terminal: record.Terminal})
}

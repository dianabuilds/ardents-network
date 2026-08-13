package serviceconn

import "errors"

func (stream *recoveryStream) sendAcknowledgements(limit uint64) error {
	for {
		stream.mu.Lock()
		if stream.terminal != nil {
			err := stream.terminal
			stream.mu.Unlock()
			return err
		}
		if stream.ackSent == limit {
			stream.mu.Unlock()
			return nil
		}
		stream.mu.Unlock()
		select {
		case <-stream.ackSignal:
		case <-stream.ctx.Done():
			return stream.ctx.Err()
		}
		for {
			stream.mu.Lock()
			offset := stream.ackPending
			already := stream.ackSent
			stream.mu.Unlock()
			if offset <= already {
				break
			}
			attachment, err := stream.attachment()
			if err != nil {
				return err
			}
			frame := connectionFrame{kind: ackFrameType, generation: attachment.generation, offset: offset}
			if err := stream.writeFrame(attachment, frame); err != nil {
				if recoverErr := stream.recoverAttachment(attachment); recoverErr != nil {
					return errors.Join(errRecoveryTerminal, err, recoverErr)
				}
				continue
			}
			stream.mu.Lock()
			if offset > stream.ackSent {
				stream.ackSent = offset
			}
			more := stream.ackPending > stream.ackSent
			stream.mu.Unlock()
			if !more {
				break
			}
		}
	}
}

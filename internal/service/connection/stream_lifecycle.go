package connection

import (
	"errors"
	"time"
)

func (stream *Stream) establishInitialAttachment() error {
	stream.mu.Lock()
	if stream.established || stream.current == nil {
		stream.mu.Unlock()
		return ErrActiveViolation
	}
	attachment := stream.current
	if verified := stream.initialAuthentication; verified != nil {
		stream.initialAuthentication = nil
		valid := stream.ctx.Err() == nil && stream.terminal == nil && verified.attachment == attachment && attachment.generation == 1 &&
			stream.sendBase == 0 && stream.sendEnd == 0 && stream.recvNext == 0 &&
			verified.peer.LocalNonce != [32]byte{} && verified.peer.PeerNonce != [32]byte{} &&
			verified.peer.LocalNonce != verified.peer.PeerNonce
		if valid {
			stream.established = true
		}
		stream.mu.Unlock()
		if !valid {
			return ErrActiveViolation
		}
		return nil
	}
	state := ContinuityExchange{Key: stream.continuity, Generation: attachment.generation,
		SendBase: stream.sendBase, SendEnd: stream.sendEnd, ReceiveNext: stream.recvNext,
		Context: attachment.context, ExporterCommitment: attachment.exporterCommitment, Role: RoleClient}
	if !stream.client {
		state.Role = RolePublisher
	}
	stream.mu.Unlock()
	peer, err := ExchangeContinuity(stream.ctx, attachment.carrier, state)
	if err != nil {
		return ErrActiveViolation
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.current != attachment || peer.ReceiveNext != stream.sendBase || peer.SendEnd != stream.recvNext ||
		peer.PeerNonce == [32]byte{} || peer.LocalNonce == [32]byte{} || peer.PeerNonce == peer.LocalNonce {
		return ErrActiveViolation
	}
	stream.established = true
	return nil
}

func (stream *Stream) outcome() Outcome {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return Outcome{Accepted: uint32(stream.sendEnd), Acknowledged: uint32(stream.sendBase),
		Received: uint32(stream.recvNext), QueueHigh: stream.queueMax, Generation: stream.currentGenerationLocked(),
		Recoveries: stream.recoveries, ContinuityCommitment: stream.continuityCommitment()}
}

func (stream *Stream) watchNameOrigin() {
	if stream.nameBinding == (DestinationBinding{}) {
		return
	}
	go func() {
		for {
			select {
			case <-stream.done:
				return
			case update, ok := <-stream.nameUpdates:
				if !ok || !ContinuesNameOrigin(stream.nameBinding, update) {
					stream.fail(errors.New("resolved Service Name binding changed"))
					return
				}
			}
		}
	}()
}

func (stream *Stream) attachment() (*Attachment, error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	for stream.recovering && stream.terminal == nil {
		stream.cond.Wait()
	}
	if stream.terminal != nil {
		return nil, stream.terminal
	}
	return stream.current, nil
}

func (stream *Stream) fail(err error) {
	if err == nil {
		return
	}
	stream.mu.Lock()
	attachment, application := stream.failLocked(err)
	stream.mu.Unlock()
	stream.releaseFailure(attachment, application)
}

// failLocked publishes a terminal failure before waking workers that could
// hand the Stream to its post-close terminal-control owner. Callers hold
// stream.mu and must release the returned resources after unlocking.
func (stream *Stream) failLocked(err error) (*Attachment, Application) {
	if err == nil {
		return nil, nil
	}
	var attachment *Attachment
	var application Application
	if stream.terminal == nil {
		stream.terminal = err
		attachment, application = stream.current, stream.application
	}
	stream.recovering = false
	stream.cond.Broadcast()
	return attachment, application
}

func (stream *Stream) releaseFailure(attachment *Attachment, application Application) {
	if attachment != nil {
		attachment.closeCarrier()
	}
	if application != nil {
		if deadline, ok := application.(interface{ SetDeadline(time.Time) error }); ok {
			_ = deadline.SetDeadline(time.Now())
		} else {
			_ = application.Close()
		}
	}
	select {
	case stream.ackSignal <- struct{}{}:
	default:
	}
}

func (stream *Stream) close() {
	stream.mu.Lock()
	if stream.current != nil {
		stream.current.closeCarrier()
	}
	stream.mu.Unlock()
	erase(stream.continuity[:])
}

func (stream *Stream) currentGenerationLocked() uint64 {
	if stream.current == nil {
		return 0
	}
	return stream.current.generation
}

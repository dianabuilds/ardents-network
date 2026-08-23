package connection

import (
	"context"
	"errors"
	"time"
)

// Run copies the exact declared byte counts in both directions, recovering
// only through the immutable AttachmentOpener contract.
func (stream *Stream) Run(sendCount, receiveCount uint32) (Outcome, error) {
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
	go func() { dataResults <- stream.sendApplication(uint64(sendCount)) }()
	go func() { dataResults <- stream.receiveApplication(uint64(receiveCount), uint64(sendCount)) }()
	go func() { ackResult <- stream.sendAcknowledgements(uint64(receiveCount)) }()
	first := <-dataResults
	if errors.Is(first, ErrActiveViolation) || errors.Is(first, errRecoveryTerminal) {
		stream.fail(first)
	}
	second := <-dataResults
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

func (stream *Stream) establishInitialAttachment() error {
	stream.mu.Lock()
	if stream.established || stream.current == nil {
		stream.mu.Unlock()
		return ErrActiveViolation
	}
	attachment := stream.current
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
	if stream.terminal == nil {
		stream.terminal = err
		if stream.current != nil {
			stream.current.closeCarrier()
		}
		if deadline, ok := stream.application.(interface{ SetDeadline(time.Time) error }); ok {
			_ = deadline.SetDeadline(time.Now())
		} else {
			_ = stream.application.Close()
		}
	}
	stream.recovering = false
	stream.cond.Broadcast()
	stream.mu.Unlock()
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

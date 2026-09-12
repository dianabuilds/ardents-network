package route

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"time"
)

// AcceptStream handles exactly one JOIN on an already admitted role TLS channel.
// It verifies the original exporter again, reserves parsing/reply memory before
// reading, and transfers successful matching into the joined stream lifecycle.
// The caller retains the connection and admission if this transfer is refused.
func (owner *ClosedJoinPairs) AcceptStream(ctx context.Context, lease *ClosedAdmission, connection net.Conn) (outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || lease == nil || connection == nil || lease.duty == nil || lease.Class != 2 {
		return errors.New("closed JOIN receiving admission unavailable")
	}
	body, err := EncodeClosedHello(lease.hello)
	if err != nil {
		return err
	}
	exporter, err := ClosedRoleTLSExporter(connection)
	if err != nil {
		return err
	}
	bindingContext := sha256.Sum256(body)
	binding, err := exporter(closedChannelExporterLabel, bindingContext[:], 32)
	if err != nil || lease.exporter == [32]byte{} || !bytes.Equal(binding, lease.exporter[:]) {
		return errors.New("closed JOIN TLS admission differs")
	}
	if err := owner.limits.queue(closedJoinStreamQueue); err != nil {
		return err
	}
	held := true
	defer func() {
		if held {
			owner.limits.dequeue(closedJoinStreamQueue)
		}
	}()
	pending := time.Now().Add(10 * time.Second)
	if lease.Deadline.Before(pending) {
		pending = lease.Deadline
	}
	if err := connection.SetDeadline(pending); err != nil {
		return err
	}
	interrupted := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); interruptErr = connection.SetDeadline(time.Now()) })
	defer func() {
		if !stop() {
			<-interrupted
			outcome = errors.Join(outcome, interruptErr)
		}
	}()
	frame, err := ReadClosedLaneFrame(connection)
	if err != nil {
		return err
	}
	defer func() { clear(frame.Body) }()
	side, err := owner.Reserve(lease, frame)
	if err != nil {
		request, decodeErr := DecodeClosedJoinRequest(frame.Body)
		clear(frame.Body)
		frame.Body = nil
		if frame.Kind == closedFrameOperation && frame.Lane == 1 && decodeErr == nil {
			refusal, encodeErr := EncodeClosedJoinResult(request.Nonce, 1)
			if encodeErr != nil {
				return errors.Join(err, encodeErr)
			}
			return errors.Join(err, WriteClosedLaneFrame(connection, ClosedLaneFrame{Kind: closedFrameResult, Lane: 1, Body: refusal}))
		}
		return err
	}
	defer side.Close()
	clear(frame.Body)
	frame.Body = nil
	owner.limits.dequeue(closedJoinStreamQueue)
	held = false
	return side.Serve(ctx, connection)
}

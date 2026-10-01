package route

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// One synchronous reader/writer per side holds enough aggregate reservation
// for the incoming frame and the encoded output, including fixed RESULT.
const closedJoinStreamQueue = 2 * (ardp.HeaderSize + ardp.MaximumBodySize)

type closedJoinStream struct {
	connection       net.Conn
	ioDone, finished chan struct{}
	credit           uint64
	eof              bool
}

// Serve transfers the actual role TLS stream to the admitted JOIN side. The
// caller must already bind that TLS exporter through the outer admission owner.
// It writes the local RESULT, forwards only framed data after both confirmations,
// and returns the admission only after both directions have joined their I/O.
func (side *ClosedJoinSide) Serve(ctx context.Context, connection net.Conn) (outcome error) {
	if side == nil || ctx == nil || connection == nil {
		return errors.New("closed JOIN stream unavailable")
	}
	owner := side.owner
	owner.mu.Lock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	if side.closed || side.pair.stopped || side.stream != nil || side.resultTaken || ctx.Err() != nil {
		owner.mu.Unlock()
		return errors.New("closed JOIN stream unavailable")
	}
	if err := owner.limits.queue(closedJoinStreamQueue); err != nil {
		owner.mu.Unlock()
		return err
	}
	stream := &closedJoinStream{connection: connection, ioDone: make(chan struct{}), finished: make(chan struct{}), credit: closedLaneCredit}
	side.stream = stream
	owner.mu.Unlock()
	interrupted := make(chan struct{})
	var closeErr error
	closeAfterIO := false
	go func() {
		defer close(interrupted)
		select {
		case <-ctx.Done():
			side.Abort()
		case <-side.Done():
		}
		// Preserve a completed JOIN's terminal status through TLS.Close, whose
		// generic underlying Close otherwise reports a refused outer lane.
		owner.mu.Lock()
		graceful := side.pair.graceful
		owner.mu.Unlock()
		if secured, ok := connection.(*tls.Conn); ok && graceful {
			if lane, ok := secured.NetConn().(*ClosedOuterBridgeLane); ok && lane.lane != nil {
				lane.lane.mu.Lock()
				lane.lane.successfulClose = true
				lane.lane.mu.Unlock()
			}
		}
		if graceful {
			// Stop the pending read without closing TLS while its opposite
			// writer is still returning. TLS.Close with an active writer can
			// skip close_notify and retire the underlying lane prematurely.
			deadline := time.Now().Add(time.Second)
			if side.wallDeadline.Before(deadline) {
				deadline = side.wallDeadline
			}
			closeErr = errors.Join(connection.SetReadDeadline(time.Now()), connection.SetWriteDeadline(deadline))
			if closeErr == nil {
				closeAfterIO = true
				return
			}
		}
		closeErr = errors.Join(closeErr, connection.Close())
	}()
	defer func() {
		side.Abort()
		<-interrupted
		outcome = errors.Join(outcome, closeErr)
		// This handler's read and opposite-side write have both returned. The other
		// handler may still be leaving its write to this connection; join it too.
		close(stream.ioDone)
		owner.mu.Lock()
		peer := side.peerLocked()
		var peerDone <-chan struct{}
		if peer != nil && peer.stream != nil {
			peerDone = peer.stream.ioDone
		}
		owner.mu.Unlock()
		if peerDone != nil {
			<-peerDone
		}
		if closeAfterIO {
			deadline := time.Now().Add(time.Second)
			if side.wallDeadline.Before(deadline) {
				deadline = side.wallDeadline
			}
			outcome = errors.Join(outcome, connection.SetWriteDeadline(deadline), connection.Close())
		}
		owner.limits.dequeue(closedJoinStreamQueue)
		side.release()
		close(stream.finished)
	}()
	if err := connection.SetDeadline(side.wallDeadline); err != nil {
		return err
	}
	if err := side.WaitPair(ctx); err != nil {
		return err
	}
	result, err := side.Result()
	if err != nil {
		return err
	}
	if err := side.writeFrame(connection, ardp.Frame{Kind: ardp.KindResult, Lane: 1, Body: result}); err != nil {
		return err
	}
	clear(result)
	result = nil
	if err := side.ConfirmResult(); err != nil {
		return err
	}
	if err := side.WaitData(ctx); err != nil {
		return err
	}
	for {
		frame, err := side.readFrame(connection)
		if err != nil {
			return err
		}
		if frame.Kind == ardp.KindAdmit {
			if err := side.replenish(frame); err != nil {
				return err
			}
			continue
		}
		peer, err := side.forwardFrame(frame)
		if err != nil {
			return err
		}
		if err := peer.writeFrame(peer.stream.connection, frame); err != nil {
			return err
		}
		if frame.Kind == ardp.KindClose {
			owner.mu.Lock()
			owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
			if side.pair.stopped || ctx.Err() != nil {
				owner.stopLocked(side.pair)
				owner.mu.Unlock()
				return errors.New("closed JOIN ended before terminal completion")
			}
			side.pair.graceful = frame.Body[0] == 0
			owner.stopLocked(side.pair)
			owner.mu.Unlock()
			if frame.Body[0] != 0 {
				return errors.New("closed JOIN peer refused")
			}
			return nil
		}
	}
}

func (side *ClosedJoinSide) peerLocked() *ClosedJoinSide {
	for _, peer := range side.pair.sides {
		if peer != nil && peer != side {
			return peer
		}
	}
	return nil
}

func (side *ClosedJoinSide) account(size uint64) error {
	owner := side.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	if side.closed || side.pair.stopped || side.used > side.byteLimit || size > side.byteLimit-side.used {
		return errors.New("closed JOIN traffic exhausted")
	}
	side.used += size
	return nil
}

func (side *ClosedJoinSide) readFrame(reader io.Reader) (ardp.Frame, error) {
	var header [ardp.HeaderSize]byte
	if err := side.account(ardp.HeaderSize); err != nil {
		return ardp.Frame{}, err
	}
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return ardp.Frame{}, err
	}
	length := binary.BigEndian.Uint32(header[12:16])
	if !ardp.ValidHeader(header[:]) {
		return ardp.Frame{}, errors.New("closed JOIN data header invalid")
	}
	if err := side.account(uint64(length)); err != nil {
		return ardp.Frame{}, err
	}
	frame := ardp.Frame{Kind: header[6], Lane: binary.BigEndian.Uint32(header[8:12]), Body: make([]byte, length)}
	if _, err := io.ReadFull(reader, frame.Body); err != nil {
		return ardp.Frame{}, err
	}
	if !ardp.ValidFrame(frame) {
		return ardp.Frame{}, errors.New("closed JOIN data frame invalid")
	}
	return frame, nil
}

func (side *ClosedJoinSide) writeFrame(writer io.Writer, frame ardp.Frame) error {
	if err := side.account(uint64(ardp.HeaderSize + len(frame.Body))); err != nil {
		return err
	}
	return ardp.WriteFrame(writer, frame)
}

func (side *ClosedJoinSide) forwardFrame(frame ardp.Frame) (*ClosedJoinSide, error) {
	owner := side.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	peer := side.peerLocked()
	if side.closed || side.pair.stopped || peer == nil || peer.stream == nil || !side.confirmed || !peer.confirmed || frame.Lane != 1 {
		return nil, errors.New("closed JOIN data inactive")
	}
	switch frame.Kind {
	case ardp.KindBytes:
		if side.stream.eof || uint64(len(frame.Body)) > side.stream.credit {
			return nil, errors.New("closed JOIN data credit exceeded")
		}
		side.stream.credit -= uint64(len(frame.Body))
	case ardp.KindCredit:
		increment := uint64(binary.BigEndian.Uint32(frame.Body))
		if increment == 0 || increment > closedLaneCredit-peer.stream.credit {
			return nil, errors.New("closed JOIN credit exceeds consumption")
		}
		peer.stream.credit += increment
	case ardp.KindEOF:
		if side.stream.eof {
			return nil, errors.New("closed JOIN repeated EOF")
		}
		side.stream.eof = true
	case ardp.KindClose:
	default:
		return nil, errors.New("closed JOIN operation after activation")
	}
	return peer, nil
}

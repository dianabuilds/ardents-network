//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// Each bootstrap leg and the source Entry-to-Interior leg has one child.
// Terminal children below the admitted Interior belong to closedSourceChannels.
// This adapter's Close retires its parent and joins its reader.
type closedRoleChildStream struct {
	writeDeadline         time.Time
	physicalWriting       bool
	payloadFrames         uint64
	physicalWriteFailed   bool
	cleanPeerClose        bool
	onTerminal            func(error)
	inputEOF              bool
	retire                func() error
	active                bool
	consumed              uint32
	parent                net.Conn
	deadline              time.Time
	writer                chan struct{}
	writeChanged          chan struct{}
	mu                    sync.Mutex
	changed               *sync.Cond
	buffer                []byte
	credit, receiveCredit uint32
	terminal              error
	done                  chan struct{}
	closeOnce             sync.Once
	closeErr              error
}

func newClosedRoleChildStream(parent net.Conn, deadline time.Time, retire func() error, onTerminal func(error)) *closedRoleChildStream {
	stream := &closedRoleChildStream{onTerminal: onTerminal, parent: parent, deadline: deadline, writeDeadline: deadline, writer: make(chan struct{}, 1), writeChanged: make(chan struct{}), retire: retire, credit: 64 << 10, receiveCredit: 64 << 10, done: make(chan struct{})}
	stream.changed = sync.NewCond(&stream.mu)
	go stream.readFrames()
	return stream
}

func (stream *closedRoleChildStream) finish(err error) {
	stream.mu.Lock()
	if stream.terminal == nil {
		stream.terminal = err
	}
	stream.signalLocked()
	stream.mu.Unlock()
}

func (stream *closedRoleChildStream) readFrames() {
	defer close(stream.done)
	defer func() {
		stream.mu.Lock()
		cause := stream.terminal
		stream.mu.Unlock()
		if stream.onTerminal != nil {
			stream.onTerminal(cause)
		}
		_ = stream.retire() // Interrupt physical I/O only after owner notification.
	}()
	for {
		frame, err := ReadClosedLaneFrame(stream.parent)
		if err != nil {
			stream.finish(err)
			return
		}
		if frame.Lane != 1 {
			stream.finish(errors.New("closed bootstrap unexpected lane"))
			return
		}
		stream.mu.Lock()
		switch frame.Kind {
		case closedFrameBytes:
			if stream.inputEOF || uint32(len(frame.Body)) > stream.receiveCredit {
				err = errors.New("closed bootstrap receive credit exceeded")
			} else {
				stream.receiveCredit -= uint32(len(frame.Body))
				stream.buffer = append(stream.buffer, frame.Body...)
			}
		case closedFrameCredit:
			increment := binary.BigEndian.Uint32(frame.Body)
			if increment > 64<<10-stream.credit {
				err = errors.New("closed bootstrap credit exceeds outstanding bytes")
			} else {
				stream.credit += increment
			}
		case closedFrameEOF:
			if stream.inputEOF {
				err = errors.New("closed bootstrap duplicate EOF")
			} else {
				stream.inputEOF = true
			}
		case closedFrameClose:
			if frame.Body[0] == 0 {
				stream.cleanPeerClose = stream.terminal == nil
				err = io.EOF
			} else {
				err = errors.New("closed bootstrap child refused")
			}
		default:
			err = errors.New("closed bootstrap child frame is invalid")
		}
		if err != nil {
			stream.terminal = err
		}
		stream.signalLocked()
		stream.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (stream *closedRoleChildStream) Read(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	stream.mu.Lock()
	for len(stream.buffer) == 0 && !stream.inputEOF && stream.terminal == nil {
		stream.changed.Wait()
	}
	if len(stream.buffer) == 0 {
		err := stream.terminal
		if err == nil && stream.inputEOF {
			err = io.EOF
		}
		stream.mu.Unlock()
		return 0, err
	}
	count := copy(value, stream.buffer)
	clear(stream.buffer[:count])
	stream.buffer = stream.buffer[count:]
	stream.consumed += uint32(count)
	stream.mu.Unlock()
	// A failed credit write must not discard bytes already returned by Read.
	// The stored terminal error is reported on the next read/write.
	_ = stream.returnCredit()
	return count, nil
}

// The receiving bridge accepts CREDIT only after authenticated inner HELLO.
// Retain consumption during TLS and release it after the role's ACCEPT.
func (stream *closedRoleChildStream) activate() error {
	stream.mu.Lock()
	stream.active = true
	stream.mu.Unlock()
	return stream.returnCredit()
}

func (stream *closedRoleChildStream) returnCredit() error {
	if err := stream.acquireWriter(true); err != nil {
		return err
	}
	defer func() { <-stream.writer }()
	stream.mu.Lock()
	if !stream.active || stream.terminal != nil || stream.consumed == 0 {
		stream.mu.Unlock()
		return nil
	}
	count := stream.consumed
	stream.consumed = 0
	stream.receiveCredit += count
	stream.mu.Unlock()
	body := binary.BigEndian.AppendUint32(nil, count)
	err := stream.writeFrame(ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: body}, true)
	if err != nil {
		stream.finish(err)
	}
	return err
}
func (stream *closedRoleChildStream) Write(value []byte) (int, error) {
	written := 0
	for len(value) > 0 {
		if err := stream.acquireWriter(false); err != nil {
			return written, err
		}
		stream.mu.Lock()
		count := min(len(value), closedLaneMaximum, int(stream.credit))
		stream.credit -= uint32(count)
		stream.mu.Unlock()
		err := stream.writeFrame(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: value[:count]}, false)
		<-stream.writer
		if err != nil {
			stream.finish(err)
			return written, err
		}
		written += count
		value = value[count:]
	}
	return written, nil
}
func (stream *closedRoleChildStream) Close() error {
	stream.closeOnce.Do(func() {
		stream.finish(net.ErrClosed)
		stream.closeErr = stream.retire()
		<-stream.done
		stream.mu.Lock()
		clear(stream.buffer)
		stream.buffer = nil
		stream.mu.Unlock()
	})
	return stream.closeErr
}

func (stream *closedRoleChildStream) bound(deadline time.Time) time.Time {
	if deadline.IsZero() || stream.deadline.Before(deadline) {
		return stream.deadline
	}
	return deadline
}
func (stream *closedRoleChildStream) SetDeadline(deadline time.Time) error {
	if err := stream.SetReadDeadline(deadline); err != nil {
		return err
	}
	return stream.SetWriteDeadline(deadline)
}
func (stream *closedRoleChildStream) SetReadDeadline(deadline time.Time) error {
	return stream.parent.SetReadDeadline(stream.bound(deadline))
}
func (stream *closedRoleChildStream) SetWriteDeadline(deadline time.Time) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.writeDeadline = stream.bound(deadline)
	stream.signalLocked()
	if stream.physicalWriting {
		return stream.parent.SetWriteDeadline(stream.writeDeadline)
	}
	return nil
}
func (stream *closedRoleChildStream) LocalAddr() net.Addr  { return stream.parent.LocalAddr() }
func (stream *closedRoleChildStream) RemoteAddr() net.Addr { return stream.parent.RemoteAddr() }

// Callers hold writer. Reapply the deadline for each physical frame: outer
// CREDIT retains the original parent bound, whereas payload writes retain
// the caller's exact deadline. Concurrent deadline updates still interrupt
// an in-flight frame; no failed physical write is normalized into success.
func (stream *closedRoleChildStream) writeFrame(frame ClosedLaneFrame, credit bool) error {
	stream.mu.Lock()
	deadline := stream.writeDeadline
	if credit {
		deadline = stream.deadline
	}
	err := stream.terminal
	attempted := err == nil
	if attempted {
		stream.physicalWriting = true
		if !credit {
			stream.payloadFrames++
		}
		err = stream.parent.SetWriteDeadline(deadline)
	}
	stream.mu.Unlock()
	if err == nil {
		err = WriteClosedLaneFrame(stream.parent, frame)
	}
	stream.mu.Lock()
	stream.physicalWriting = false
	if attempted && err != nil {
		stream.physicalWriteFailed = true
	}
	stream.signalLocked()
	stream.mu.Unlock()
	return err
}

func (stream *closedRoleChildStream) signalLocked() {
	stream.changed.Broadcast()
	close(stream.writeChanged)
	stream.writeChanged = make(chan struct{})
}

// Waiting for the serializer or receive credit consumes the payload's original
// write deadline too. A queued timeout emits no bytes and leaves the active
// control frame with its own reservation and deadline.
func (stream *closedRoleChildStream) acquireWriter(credit bool) error {
	for {
		stream.mu.Lock()
		deadline := stream.writeDeadline
		if credit {
			deadline = stream.deadline
		}
		err := stream.terminal
		if err == nil && !time.Now().Before(deadline) {
			err = os.ErrDeadlineExceeded
		}
		changed := stream.writeChanged
		var writer chan struct{}
		if credit || stream.credit != 0 {
			writer = stream.writer
		}
		stream.mu.Unlock()
		if err != nil {
			return err
		}
		timer := time.NewTimer(time.Until(deadline))
		select {
		case writer <- struct{}{}:
			timer.Stop()
			stream.mu.Lock()
			current := stream.writeDeadline
			if credit {
				current = stream.deadline
			}
			err = stream.terminal
			if err == nil && !time.Now().Before(current) {
				err = os.ErrDeadlineExceeded
			}
			ready := credit || stream.credit != 0
			stream.mu.Unlock()
			if err == nil && ready {
				return nil
			}
			<-stream.writer
			if err != nil {
				return err
			}
		case <-changed:
			timer.Stop()
		case <-timer.C:
			// Recheck a concurrent deadline extension before returning a timeout.
		}
	}
}

// Count every payload attempt below all nested TLS records, including a
// zero-byte failure. Independently successful CREDIT cannot imply that this
// payload started; any failed physical write, including CREDIT, vetoes clean
// evidence permanently. A successful peer CLOSE(0) is still required.
func (stream *closedRoleChildStream) writeWitness() (uint64, bool, bool) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.payloadFrames, stream.physicalWriting, stream.cleanPeerClose && stream.terminal == io.EOF && !stream.physicalWriteFailed
}

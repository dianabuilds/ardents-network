//go:build linux

package route

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

type joinedFailureSplitConn struct {
	net.Conn
	reader net.Conn
}

func (connection *joinedFailureSplitConn) Read(value []byte) (int, error) {
	return connection.reader.Read(value)
}

// Exercise the joined framing through a real Source lane. The peer consumes
// outer frames but grants no inner credit; cancellation must unblock both layers.
func joinedClientStreamFixture(t *testing.T) (*ClosedJoinedStream, context.CancelFunc, <-chan struct{}, <-chan ClosedLaneFrame, net.Conn) {
	t.Helper()
	owner, peer, end := sourceChannelsFixture(t)
	frames := make(chan ClosedLaneFrame, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			frame, err := ReadClosedLaneFrame(peer)
			if err != nil {
				return
			}
			if frame.Kind == closedFrameBytes {
				frames <- frame
				if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, uint32(len(frame.Body)))}); err != nil {
					return
				}
			}
		}
	}()
	t.Cleanup(func() { _ = peer.Close(); <-done })
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	released := make(chan struct{})
	stream := newClosedJoinedStream(ctx, lane, lane, func() { close(released) })
	t.Cleanup(func() { cancel(); _ = stream.Close() })
	return stream, cancel, released, frames, peer
}

func TestClosedJoinedClientCancellationJoinsBlockedCreditWriter(t *testing.T) {
	stream, cancel, released, frames, _ := joinedClientStreamFixture(t)
	written := make(chan error, 1)
	go func() { _, err := stream.Write(make([]byte, 128<<10)); written <- err }()
	// Return only outer credit. Four complete inner frames consume the whole
	// joined window, so the fifth cannot start before cancellation.
	timeout := time.After(2 * time.Second)
	for received := 0; received < 4*(closedLaneMaximum+closedLaneHeaderSize); {
		select {
		case frame := <-frames:
			received += len(frame.Body)
		case <-timeout:
			t.Fatal("joined writer did not exhaust its credit window")
		}
	}
	waitSourceChannelState(t, stream.channels, func() bool {
		return stream.credit == 0 && stream.channels.active == nil && len(stream.channels.data) == 0
	})
	cancel()
	select {
	case err := <-written:
		if err == nil {
			t.Fatal("canceled writer succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("credit writer leaked")
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("joined queue reservation leaked")
	}
	if _, err := stream.Read(make([]byte, 1)); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("canceled read: %v", err)
	}
}

func TestClosedJoinedClientAcceptsOuterLaneBeforeTransfer(t *testing.T) {
	stream, _, _, _, _ := joinedClientStreamFixture(t)
	stream.outer.owner.mu.Lock()
	status := stream.outer.closeStatus
	stream.outer.owner.mu.Unlock()
	if status != 0 {
		t.Fatalf("accepted JOIN retained refusal status %d", status)
	}
}

func TestClosedJoinedPeerCloseJoinsLaneBeforeReleasingOuter(t *testing.T) {
	stream, _, released, _, peer := joinedClientStreamFixture(t)
	body, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}})
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range []ClosedLaneFrame{
		{Kind: closedFrameBytes, Lane: stream.outer.id, Body: body},
		{Kind: closedFrameClose, Lane: stream.outer.id, Body: []byte{0}},
	} {
		if err := WriteClosedLaneFrame(peer, frame); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("clean peer close did not release joined reservation")
	}
	stream.channels.mu.Lock()
	closed := stream.closedSourceLane.closed
	stream.channels.mu.Unlock()
	if !closed {
		t.Fatal("outer transport retired before joined lane cleanup")
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedJoinedReaderFailureInterruptsBlockedWriter(t *testing.T) {
	for _, mode := range []string{"malformed", "raw-eof"} {
		t.Run(mode, func(t *testing.T) { checkClosedJoinedReaderFailureInterruptsBlockedWriter(t, mode) })
	}
}

func checkClosedJoinedReaderFailureInterruptsBlockedWriter(t *testing.T, mode string) {
	owner, peer, end := sourceChannelsFixture(t)
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader, readerPeer := net.Pipe()
	defer reader.Close()
	readerPeerClosed := false
	defer func() {
		if !readerPeerClosed {
			_ = readerPeer.Close()
		}
	}()
	released := make(chan struct{})
	stream := newClosedJoinedStream(ctx, &joinedFailureSplitConn{Conn: lane, reader: reader}, lane, func() { close(released) })
	written := make(chan error, 1)
	go func() { _, writeErr := stream.Write([]byte{1}); written <- writeErr }()
	waitSourceChannelState(t, owner, func() bool {
		return owner.active != nil && owner.active.frame.Kind == closedFrameBytes
	})
	if mode == "raw-eof" {
		if err := readerPeer.Close(); err != nil {
			t.Fatal(err)
		}
		readerPeerClosed = true
	} else {
		invalid, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1})
		if err != nil {
			t.Fatal(err)
		}
		invalid[7] = 1
		if _, err := readerPeer.Write(invalid); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-written:
		if err == nil {
			t.Fatalf("writer succeeded after %s nested failure", mode)
		}
	case <-time.After(2 * time.Second):
		stream.channels.mu.Lock()
		innerCause := stream.channels.terminal
		stream.channels.mu.Unlock()
		owner.mu.Lock()
		outerCause, outerActive, outerClosed := owner.terminal, owner.active != nil, lane.closed
		owner.mu.Unlock()
		t.Fatalf("%s reader failure did not interrupt blocked writer: inner=%v outer=%v active=%t closed=%t", mode, innerCause, outerCause, outerActive, outerClosed)
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("reader failure leaked joined reservation")
	}
	if err := stream.Close(); err == nil {
		t.Fatalf("%s nested failure disappeared from cleanup", mode)
	}
}

func TestClosedJoinedClientCloseCannotExceedAdmissionBudget(t *testing.T) {
	stream, _, released, frames, _ := joinedClientStreamFixture(t)
	stream.channels.mu.Lock()
	stream.channels.transferred = 32 << 20
	stream.channels.mu.Unlock()
	if err := stream.Close(); err == nil {
		t.Error("terminal CLOSE exceeded original class-2 reserve")
	}
	<-released
	select {
	case frame := <-frames:
		t.Errorf("over-budget inner frame emitted: %d bytes", len(frame.Body))
	default:
	}
}

func TestClosedJoinedClientDrainsCleanCloseBeforeReleasingBytes(t *testing.T) {
	for _, abandon := range []bool{false, true} {
		name := "read"
		if abandon {
			name = "cancel"
		}
		t.Run(name, func(t *testing.T) {
			stream, cancel, released, _, peer := joinedClientStreamFixture(t)
			for _, frame := range []ClosedLaneFrame{{Kind: closedFrameBytes, Lane: 1, Body: []byte("final authenticated record")}, {Kind: closedFrameClose, Lane: 1, Body: []byte{0}}} {
				body, err := EncodeClosedLaneFrame(frame)
				if err != nil {
					t.Fatal(err)
				}
				if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: body}); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-stream.channels.done:
			case <-time.After(time.Second):
				t.Fatal("physical close did not join")
			}
			select {
			case <-released:
				t.Fatal("unread bytes lost their reservation")
			default:
			}
			if abandon {
				cancel()
			} else {
				body, err := io.ReadAll(stream)
				if err != nil || string(body) != "final authenticated record" {
					t.Fatalf("final bytes: %q %v", body, err)
				}
			}
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("finished reader retained reservation")
			}
			if err := stream.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedJoinedClientDoesNotHideMalformedFrameAfterCleanClose(t *testing.T) {
	stream, _, released, _, peer := joinedClientStreamFixture(t)
	for _, frame := range []ClosedLaneFrame{{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}, {Kind: closedFrameClose, Lane: 1, Body: []byte{0}}} {
		body, err := EncodeClosedLaneFrame(frame)
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("protocol failure did not release reservation")
	}
	_, err := stream.Read(make([]byte, 1))
	if err == nil || err == io.EOF {
		t.Fatalf("duplicate CLOSE was hidden: %v", err)
	}
}

func TestClosedJoinedClientTransportEOFWaitsForOuterTerminal(t *testing.T) {
	stream, _, released, _, peer := joinedClientStreamFixture(t)
	// This real framed EOF produces the same io.EOF retirement path as a clean
	// role TLS close_notify. Keep the outer terminal behind a separate barrier.
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}); err != nil {
		t.Fatal(err)
	}
	waitSourceChannelState(t, stream.channels, func() bool { return stream.channels.terminal == io.EOF })
	<-time.After(50 * time.Millisecond)
	stream.outer.owner.mu.Lock()
	retired := stream.outer.closed
	stream.outer.owner.mu.Unlock()
	if retired {
		t.Error("local retirement overtook the peer outer terminal")
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("outer terminal failed to release JOIN")
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosedJoinedClientDrainsBytesBeforeUnexpectedTransportEOF(t *testing.T) {
	stream, _, released, _, peer := joinedClientStreamFixture(t)
	body, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte("final authenticated record")})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	// Complete outer closure follows the bytes, but no inner CLOSE was sent.
	// The accepted bytes precede the unexpected-EOF error, never replace it.
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stream.channels.done:
	case <-time.After(time.Second):
		t.Fatal("physical owner did not join")
	}
	got, readErr := io.ReadAll(stream)
	if string(got) != "final authenticated record" {
		t.Fatalf("accepted bytes discarded: %q (%v)", got, readErr)
	}
	if readErr == nil || readErr == io.EOF || !errors.Is(readErr, net.ErrClosed) {
		t.Fatalf("unexpected transport EOF became clean closure: %v", readErr)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("drained bytes retained reservation")
	}
}

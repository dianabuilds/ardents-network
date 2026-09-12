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

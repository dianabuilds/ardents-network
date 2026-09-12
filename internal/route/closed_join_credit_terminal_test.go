//go:build linux

package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// Pause only reads after the first inner frame. All frame writes and the EOF
// returned by the already closed outer lane use the real framing owners.
type joinedReadBoundary struct {
	net.Conn
	remaining int
	entered   chan struct{}
	release   chan struct{}
	once      sync.Once
}

func (connection *joinedReadBoundary) Read(value []byte) (int, error) {
	if connection.remaining == 0 {
		connection.once.Do(func() { close(connection.entered); <-connection.release })
	} else if len(value) > connection.remaining {
		value = value[:connection.remaining]
	}
	count, err := connection.Conn.Read(value)
	if connection.remaining > 0 {
		connection.remaining -= count
	}
	return count, err
}

func TestClosedJoinedLateCreditCannotDiscardQueuedTerminal(t *testing.T) {
	for _, mode := range []string{"clean", "refused", "raw-eof", "partial"} {
		t.Run(mode, func(t *testing.T) { checkJoinedLateCredit(t, mode) })
	}
}

func checkJoinedLateCredit(t *testing.T, mode string) {
	t.Helper()
	outer, peer, end := sourceChannelsFixture(t)
	peerDone := make(chan struct{})
	partial := make(chan struct{})
	go func() {
		defer close(peerDone)
		if mode == "partial" {
			// OPEN, then CREDIT for the first inner header and body.
			for range 3 {
				if _, err := ReadClosedLaneFrame(peer); err != nil {
					return
				}
			}
			var first [1]byte
			if _, err := io.ReadFull(peer, first[:]); err == nil {
				close(partial)
			}
			return
		}
		for {
			if _, err := ReadClosedLaneFrame(peer); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		<-peerDone
	})
	lane, err := outer.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := lane.activate(); err != nil {
		t.Fatal(err)
	}
	first := []byte("first")
	last := []byte(" final authenticated record")
	held := &joinedReadBoundary{Conn: lane, remaining: closedLaneHeaderSize + len(first), entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	released := make(chan struct{})
	stream := newClosedJoinedStream(ctx, held, lane, func() { close(released) })
	var unblock sync.Once
	var readers sync.WaitGroup
	var cleanupCause error
	matchesFailure := func(err error) bool {
		// The real peer close races the already partial write with the parent's
		// read. Either physical failure can win; neither may become success.
		if mode == "partial" {
			return errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe)
		}
		return errors.Is(err, cleanupCause)
	}
	checkCleanup := func(err error) {
		if cleanupCause == nil {
			if err != nil {
				t.Error(err)
			}
		} else if !matchesFailure(err) {
			t.Errorf("cleanup lost original %s failure: %v", mode, err)
		}
	}
	t.Cleanup(func() {
		defer readers.Wait()
		unblock.Do(func() { close(held.release) })
		cancel()
		checkCleanup(stream.Close())
		select {
		case <-released:
		default:
			t.Error("cleanup returned before reservation release")
		}
	})
	var encoded []byte
	for _, frame := range []ClosedLaneFrame{{Kind: closedFrameBytes, Lane: 1, Body: first}, {Kind: closedFrameBytes, Lane: 1, Body: last}, {Kind: closedFrameClose, Lane: 1, Body: []byte{0}}} {
		raw, err := EncodeClosedLaneFrame(frame)
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, raw...)
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: encoded}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-held.entered:
	case <-time.After(time.Second):
		t.Fatal("inner reader did not reach final-record boundary")
	}
	prefix := make([]byte, len(first))
	readFirst := make(chan error, 1)
	if mode == "partial" {
		readers.Go(func() { _, err := io.ReadFull(stream, prefix); readFirst <- err })
		select {
		case <-partial:
		case <-time.After(time.Second):
			t.Fatal("CREDIT did not emit its physical prefix")
		}
	}
	if mode == "raw-eof" {
		if err := peer.Close(); err != nil {
			t.Fatal(err)
		}
		waitSourceChannelState(t, outer, func() bool { return outer.terminal != nil })
		cleanupCause = io.EOF
	} else {
		status := byte(0)
		if mode == "refused" {
			status = 1
		}
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{status}}); err != nil {
			t.Fatal(err)
		}
		waitSourceChannelState(t, outer, func() bool { return lane.remoteClosed })
		if mode == "refused" {
			outer.mu.Lock()
			cleanupCause = lane.failure
			outer.mu.Unlock()
		}
	}
	// Consuming the first frame now returns CREDIT through the already closed
	// outer lane. This cannot emit any bytes, and must not discard its unread tail.
	if mode == "partial" {
		cleanupCause = io.ErrClosedPipe
		if err := peer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-readFirst; err != nil {
			t.Fatal(err)
		}
	} else if _, err := io.ReadFull(stream, prefix); err != nil {
		t.Fatal(err)
	}
	unblock.Do(func() { close(held.release) })
	suffix, readErr := io.ReadAll(stream)
	got := append(prefix, suffix...)
	checkCleanup(stream.Close())
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("completed JOIN retained its reservation")
	}
	if mode != "clean" {
		if !matchesFailure(readErr) {
			t.Fatalf("%s lost its read failure: %q: %v", mode, got, readErr)
		}
	} else if string(got) != "first final authenticated record" || readErr != nil {
		t.Fatalf("late CREDIT discarded queued final records: %q: %v", got, readErr)
	}
	if err := outer.Close(); err != nil {
		t.Fatal(err)
	}
}

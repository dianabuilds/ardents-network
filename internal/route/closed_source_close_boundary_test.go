//go:build linux

package route

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// Pause the upper writer after its deadline succeeds but before frame emission.
// A real peer terminal arrives in that interval; no write error is injected.
type sourceDeadlineBoundary struct {
	net.Conn
	armed            atomic.Bool
	entered, release chan struct{}
}

func (connection *sourceDeadlineBoundary) SetWriteDeadline(end time.Time) error {
	err := connection.Conn.SetWriteDeadline(end)
	if err == nil && connection.armed.CompareAndSwap(true, false) {
		close(connection.entered)
		<-connection.release
	}
	return err
}

func TestClosedSourceParentTerminalBeforeCloseEmission(t *testing.T) {
	for _, mode := range []string{"unwritten", "partial", "refused", "raw-eof"} {
		t.Run(mode, func(t *testing.T) { checkSourceCloseBoundary(t, mode) })
	}
}

func checkSourceCloseBoundary(t *testing.T, mode string) {
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)
	retirement := &closedRoleRetirement{transport: local}
	prefix := &ClosedSourcePrefix{retirement: retirement, stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
	child := prefix.newChild(local, end)
	held := &sourceDeadlineBoundary{Conn: child, entered: make(chan struct{}), release: make(chan struct{})}
	prefix.connection, prefix.child = held, child
	prefix.interruptMu.Lock()
	prefix.channels = newClosedSourceChannelOwner(held, end, retirement.close)
	prefix.channels.framing = child
	prefix.channels.start()
	prefix.interruptMu.Unlock()
	go prefix.finishAfterChannels()
	defer prefix.Close()
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := prefix.channels.open(context.Background(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	held.armed.Store(mode != "partial")
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	if mode == "partial" {
		var emitted [1]byte
		if _, err := io.ReadFull(peer, emitted[:]); err != nil {
			t.Fatal(err)
		}
	} else {
		select {
		case <-held.entered:
		case <-time.After(time.Second):
			t.Fatal("CLOSE writer did not reach deadline boundary")
		}
	}
	if mode == "raw-eof" {
		if err := peer.Close(); err != nil {
			close(held.release)
			t.Fatal(err)
		}
	} else {
		status := byte(0)
		if mode == "refused" {
			status = 1
		}
		if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{status}}); err != nil {
			close(held.release)
			t.Fatal(err)
		}
	}
	for {
		child.mu.Lock()
		terminal, changed := child.terminal, child.writeChanged
		child.mu.Unlock()
		if terminal != nil {
			break
		}
		select {
		case <-changed:
		case <-time.After(time.Second):
			close(held.release)
			t.Fatal("peer terminal was not observed")
		}
	}
	close(held.release)
	outcome := <-closed
	if mode != "unwritten" {
		if !errors.Is(outcome, ErrClosedSourceCleanup) {
			t.Fatalf("genuine %s failure disappeared: %v", mode, outcome)
		}
		return
	}
	if outcome != nil {
		t.Fatalf("unemitted CLOSE became cleanup failure: %v", outcome)
	}
	prefix.channels.mu.Lock()
	cause := prefix.channels.terminal
	prefix.channels.mu.Unlock()
	if !errors.Is(cause, io.EOF) {
		t.Fatalf("original terminal disappeared: %v", cause)
	}
}

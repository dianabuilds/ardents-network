//go:build linux

package client

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
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

// sourceCloseWriteBoundary pauses after the source writer records its terminal
// attempt, while the lower child has not yet begun its physical write.
type sourceCloseWriteBoundary struct {
	net.Conn
	armed            atomic.Bool
	entered, release chan struct{}
}

func (connection *sourceCloseWriteBoundary) Write(value []byte) (int, error) {
	if connection.armed.CompareAndSwap(true, false) {
		close(connection.entered)
		<-connection.release
	}
	return connection.Conn.Write(value)
}

func TestClosedSourceParentTerminalBeforeCloseEmission(t *testing.T) {
	for _, mode := range []string{"unwritten", "partial", "refused", "raw-eof"} {
		t.Run(mode, func(t *testing.T) { checkSourceCloseBoundary(t, mode) })
	}
}

func TestClosedSourcePeerRefusalDuringTerminalAttempt(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)
	retirement := &closedRoleRetirement{transport: local}
	prefix := &ClosedSourcePrefix{retirement: retirement, stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
	child := prefix.newChild(local, end)
	held := &sourceCloseWriteBoundary{Conn: child, entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	prefix.connection, prefix.child = held, child
	prefix.interruptMu.Lock()
	prefix.channels = newClosedSourceChannelOwner(held, end, retirement.close)
	prefix.channels.framing = child
	prefix.channels.start()
	prefix.interruptMu.Unlock()
	go prefix.finishAfterChannels()
	defer prefix.Close()
	defer release.Do(func() { close(held.release) })
	opened := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
	lane, err := prefix.channels.open(context.Background(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	held.armed.Store(true)
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	select {
	case <-held.entered:
	case <-time.After(time.Second):
		t.Fatal("terminal writer did not reach attempted-write boundary")
	}
	prefix.channels.mu.Lock()
	active := prefix.channels.active
	attemptedClose := active != nil && active.frame.Kind == ardp.KindClose && active.attempted
	prefix.channels.mu.Unlock()
	if !attemptedClose {
		t.Fatal("write boundary did not hold an attempted terminal CLOSE")
	}
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	for {
		child.mu.Lock()
		terminal, changed := child.terminal, child.writeChanged
		child.mu.Unlock()
		if terminal != nil {
			if !strings.Contains(terminal.Error(), "closed bootstrap child refused") {
				t.Fatalf("lower child observed another terminal cause: %v", terminal)
			}
			break
		}
		select {
		case <-changed:
		case <-time.After(time.Second):
			t.Fatal("peer refusal was not observed")
		}
	}
	release.Do(func() { close(held.release) })
	select {
	case outcome := <-closed:
		if !errors.Is(outcome, ErrClosedSourceCleanup) || !strings.Contains(outcome.Error(), "closed bootstrap child refused") {
			t.Fatalf("attempted terminal write lost peer refusal: %v", outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("terminal cleanup did not join")
	}
}

func TestClosedSourceNestedCloseJoinsFailedQueueParent(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	parent := &closedSourceChannels{changed: make(chan struct{})}
	owner.queueParent = parent
	owner.retainClosedRead = true
	opened := make(chan error, 1)
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
	lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	parent.mu.Lock()
	parent.terminal = ErrClosedSourceStopped
	parent.mu.Unlock()
	if err := lane.Close(); err != nil {
		t.Fatalf("failed outer queue became inner cleanup failure: %v", err)
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
	go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
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
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
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

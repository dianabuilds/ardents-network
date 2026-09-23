//go:build linux

package route

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

// Pause the physical CLOSE after the channel writer has recorded its attempt.
// This differs from sourceDeadlineBoundary: the peer can now distinguish a
// refusal that arrived before an attempted local frame from one concurrent
// with an actual write.
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

// The initial-publication cancellation path ultimately closes a retained
// Source lane. Keep the three terminal orderings explicit: a peer refusal
// before Route records a local attempt, a refusal during a recorded attempt,
// and a raw transport retirement. The first two remain distinct genuine
// refusals; the last must not acquire a synthetic refusal classification.
func TestClosedSourceTerminalClassificationAtCloseAttemptBoundary(t *testing.T) {
	for _, mode := range []string{"refusal-before-attempt", "refusal-during-attempt", "raw-transport"} {
		t.Run(mode, func(t *testing.T) { checkSourceCloseAttemptBoundary(t, mode) })
	}
}

func checkSourceCloseAttemptBoundary(t *testing.T, mode string) {
	t.Helper()
	local, peer := net.Pipe()
	defer peer.Close()
	end := time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)
	retirement := &closedRoleRetirement{transport: local}
	prefix := &ClosedSourcePrefix{retirement: retirement, stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
	child := prefix.newChild(local, end)
	deadline := &sourceDeadlineBoundary{Conn: child, entered: make(chan struct{}), release: make(chan struct{})}
	writer := &sourceCloseWriteBoundary{Conn: child, entered: make(chan struct{}), release: make(chan struct{})}
	var releaseDeadline, releaseWriter sync.Once
	t.Cleanup(func() {
		releaseDeadline.Do(func() { close(deadline.release) })
		releaseWriter.Do(func() { close(writer.release) })
	})
	connection := net.Conn(deadline)
	if mode == "refusal-during-attempt" {
		connection = writer
	}
	prefix.connection, prefix.child = connection, child
	prefix.interruptMu.Lock()
	prefix.channels = newClosedSourceChannelOwner(connection, end, retirement.close)
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
	if mode == "refusal-during-attempt" {
		writer.armed.Store(true)
	} else {
		deadline.armed.Store(true)
	}
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	entered := deadline.entered
	if mode == "refusal-during-attempt" {
		entered = writer.entered
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("CLOSE writer did not reach selected boundary")
	}
	// sourceDeadlineBoundary runs before SetWriteDeadline returns, while the
	// writer still holds owner.mu and before it sets request.attempted. The
	// physical Write boundary runs only after that assignment and unlock.
	if mode == "raw-transport" {
		if err := peer.Close(); err != nil {
			t.Fatal(err)
		}
	} else if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if mode == "refusal-during-attempt" {
		releaseWriter.Do(func() { close(writer.release) })
	} else {
		releaseDeadline.Do(func() { close(deadline.release) })
	}
	select {
	case outcome := <-closed:
		if !errors.Is(outcome, ErrClosedSourceCleanup) {
			t.Fatalf("terminal cleanup lost %s failure: %v", mode, outcome)
		}
		if mode == "raw-transport" && strings.Contains(outcome.Error(), "child refused") {
			t.Fatalf("raw transport error acquired refusal classification: %v", outcome)
		}
		if mode != "raw-transport" && !strings.Contains(outcome.Error(), "child refused") {
			t.Fatalf("peer refusal disappeared at %s: %v", mode, outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CLOSE did not join terminal boundary")
	}
}

func TestClosedSourceNestedCloseJoinsFailedQueueParent(t *testing.T) {
	owner, peer, end := sourceChannelsFixture(t)
	parent := &closedSourceChannels{changed: make(chan struct{})}
	owner.queueParent = parent
	owner.retainClosedRead = true
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
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

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

// Hold the upper reader's terminal observation while the nested framing
// reader has already finished. No socket error or retirement is simulated.
type sourceDelayedTerminal struct {
	net.Conn
	entered, release chan struct{}
	deadlineFailure  chan error
	once             sync.Once
}

func (connection *sourceDelayedTerminal) Read(body []byte) (int, error) {
	n, err := connection.Conn.Read(body)
	if err != nil {
		connection.once.Do(func() { close(connection.entered); <-connection.release })
	}
	return n, err
}
func (connection *sourceDelayedTerminal) SetWriteDeadline(end time.Time) error {
	err := connection.Conn.SetWriteDeadline(end)
	if err != nil {
		select {
		case connection.deadlineFailure <- err:
		default:
		}
	}
	return err
}

func TestClosedSourceNestedTerminalPrecedesPhysicalRetirement(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	local, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	peer, err := listener.Accept()
	listener.Close()
	if err != nil {
		local.Close()
		t.Fatal(err)
	}
	defer peer.Close()
	end := time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)
	retirement := &closedRoleRetirement{transport: local}
	prefix := &ClosedSourcePrefix{retirement: retirement, stop: func() bool { return true }, interrupted: make(chan struct{}), done: make(chan struct{})}
	child := prefix.newChild(local, end)
	held := &sourceDelayedTerminal{Conn: child, entered: make(chan struct{}), release: make(chan struct{}), deadlineFailure: make(chan error, 1)}
	prefix.connection, prefix.child = held, child
	prefix.interruptMu.Lock()
	prefix.channels = newClosedSourceChannelOwner(held, end, retirement.close)
	prefix.channels.start()
	prefix.interruptMu.Unlock()
	go prefix.finishAfterChannels()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(held.release) }) }
	defer func() { release(); prefix.Close() }()
	opened := make(chan error, 1)
	go func() { _, err := ReadClosedLaneFrame(peer); opened <- err }()
	lane, err := prefix.channels.open(context.Background(), sourceIssuerOpen(end), end)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
	if err := WriteClosedLaneFrame(peer, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-child.done:
	case <-time.After(time.Second):
		t.Fatal("nested reader did not retire physical TCP")
	}
	select {
	case <-held.entered:
	case <-time.After(time.Second):
		t.Fatal("upper terminal read not held")
	}
	closed := make(chan error, 1)
	go func() { closed <- lane.Close() }()
	var outcome error
	select {
	case outcome = <-closed:
		release()
	case err := <-held.deadlineFailure:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("unexpected physical deadline error: %v", err)
		}
		release()
		outcome = <-closed
	case <-time.After(time.Second):
		release()
		t.Fatal("child close stalled without terminal or physical error")
	}
	if outcome != nil {
		t.Fatalf("nested terminal caused false child cleanup failure: %v", outcome)
	}
	prefix.channels.mu.Lock()
	cause := prefix.channels.terminal
	prefix.channels.mu.Unlock()
	if !errors.Is(cause, io.EOF) {
		t.Fatalf("original nested terminal cause lost: %v", cause)
	}
	if err := prefix.Close(); err != nil {
		t.Fatal(err)
	}
}

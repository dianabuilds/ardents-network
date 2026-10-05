package channel

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Hold only completion of real net.Pipe output. No successful authority or
// admission is substituted; the exact timeout comes from interrupted I/O.
type peerCloseWriterConn struct {
	net.Conn
	entered, returned, completion chan struct{}
}

func (c *peerCloseWriterConn) Write(body []byte) (int, error) {
	close(c.entered)
	n, err := c.Conn.Write(body)
	close(c.returned)
	<-c.completion
	return n, err
}

func TestIncomingCLOSEInterruptsStartedWriterAndJoinsBeforeRelease(t *testing.T) {
	local, peer := net.Pipe()
	physical := &peerCloseWriterConn{Conn: local, entered: make(chan struct{}), returned: make(chan struct{}), completion: make(chan struct{})}
	queues := &Budget{maximum: 64 << 20}
	release, err := queues.HoldControl()
	if err != nil {
		t.Fatal(err)
	}
	s := New(context.Background(), physical, time.Now().Add(10*time.Second), 32<<20, nil, false, queues, nil)
	l := lifecycleLane(t, s, 1)
	var once sync.Once
	unblock := func() { once.Do(func() { close(physical.completion) }) }
	t.Cleanup(func() { unblock(); _ = peer.Close(); _ = s.Close() })
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("started physical frame")); written <- err }()
	<-physical.entered
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-physical.returned:
	case <-time.After(time.Second):
		t.Fatal("peer CLOSE did not interrupt its active physical writer")
	}
	joined := make(chan error, 1)
	go func() { err := s.Close(); release(); joined <- err }()
	select {
	case err := <-joined:
		t.Fatal("released before actual writer completion", err)
	default:
	}
	queues.mu.Lock()
	held := queues.used
	queues.mu.Unlock()
	if held < 16<<10 {
		t.Fatal("original control capacity released before join", held)
	}
	unblock()
	if err := lifecycleResult(t, written); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("actual interrupted I/O cause lost", err)
	}
	first := lifecycleResult(t, joined)
	if !errors.Is(first, os.ErrDeadlineExceeded) || s.Close() != first {
		t.Fatal("joined physical result lost or replaced", first)
	}
	var failure *physicalWriteFailure
	if !errors.As(s.PhysicalFailure(), &failure) || failure.kind != ardp.KindBytes {
		t.Fatal("started physical provenance lost", s.PhysicalFailure())
	}
	queues.mu.Lock()
	held = queues.used
	queues.mu.Unlock()
	if held != 0 {
		t.Fatal("joined physical capacity leaked", held)
	}
}

package source

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// Only holds return after the real original TCP Close. It neither supplies successful
// transport/authentication nor replaces the actual native close operation.
type heldSourceClose struct {
	net.Conn
	entered   chan struct{}
	release   chan struct{}
	completed chan struct{}
	once      sync.Once
	err       error
}

func (c *heldSourceClose) Close() error {
	c.once.Do(func() { defer close(c.completed); c.err = c.Conn.Close(); close(c.entered); <-c.release })
	return c.err
}

func TestSourceCancellationJoinsOriginalNativeClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	peer, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	held := &heldSourceClose{Conn: client, entered: make(chan struct{}), release: make(chan struct{}), completed: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(held.release) })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	retire := sourceConnectionCancellation(ctx, held)
	cancel()
	<-held.entered
	// Independently observe the actual socket close. Its effect is not enough:
	// the still-running original callback must also physically return.
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var raw [1]byte
	if _, err := peer.Read(raw[:]); !errors.Is(err, io.EOF) {
		t.Fatal("actual original socket did not close", err)
	}
	started, joined := make(chan struct{}), make(chan struct{})
	go func() { close(started); retire(); close(joined) }()
	defer func() { release.Do(func() { close(held.release) }); <-held.completed; <-joined }()
	<-started
	select {
	case <-joined:
		t.Fatal("original callback/connection ownership returned before native Close")
	case <-time.After(25 * time.Millisecond): // Negative observation only, never completion.
	}
	release.Do(func() { close(held.release) })
	<-joined // Actual original callback completion, not cancellation or timeout.
	if held.err != nil {
		t.Fatal("actual original TCP Close", held.err)
	}
}

func TestSourceCancellationNormalRetirementStopsOriginalCallback(t *testing.T) {
	connection, peer := net.Pipe()
	defer connection.Close()
	defer peer.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	retire := sourceConnectionCancellation(ctx, connection)
	retire()
	cancel()
	// The stopped callback cannot close a still-owned normal connection.
	written := make(chan error, 1)
	go func() { _, err := connection.Write([]byte{1}); written <- err }()
	writeJoined := false
	defer func() {
		if !writeJoined {
			_ = connection.Close()
			_ = peer.Close()
			<-written
		}
	}()
	var raw [1]byte
	if _, err := peer.Read(raw[:]); err != nil || raw[0] != 1 {
		t.Fatal("normal ownership was closed", err)
	}
	err := <-written
	writeJoined = true
	if err != nil {
		t.Fatal(err)
	}
}

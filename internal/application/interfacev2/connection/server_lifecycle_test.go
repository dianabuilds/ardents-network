//go:build linux

package connection_test

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

type waitingOwner struct {
	entered chan context.Context
	stream  *waitingStream
}

func (owner *waitingOwner) Open(ctx context.Context, _ connection.Request) (connection.Stream, error) {
	owner.entered <- ctx
	if owner.stream != nil {
		return owner.stream, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

type waitingStream struct {
	closed   chan struct{}
	done     chan connection.Outcome
	once     sync.Once
	closeErr error
}

func (stream *waitingStream) Read([]byte) (int, error)        { <-stream.closed; return 0, io.EOF }
func (stream *waitingStream) Write([]byte) (int, error)       { <-stream.closed; return 0, net.ErrClosed }
func (stream *waitingStream) CloseInput() error               { return nil }
func (stream *waitingStream) Done() <-chan connection.Outcome { return stream.done }
func (stream *waitingStream) Close() error {
	stream.once.Do(func() {
		close(stream.closed)
		stream.done <- connection.Outcome{Class: connection.LocalCancellation}
		close(stream.done)
	})
	return stream.closeErr
}

func TestServerJoinsBlockedApplicationOnClose(t *testing.T) {
	stream := &waitingStream{closed: make(chan struct{}), done: make(chan connection.Outcome, 1)}
	owner := &waitingOwner{entered: make(chan context.Context, 1), stream: stream}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := connection.Dial(t.Context(), path, connection.Request{Destination: connection.TargetLink, Value: "explicit"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	closed := make(chan error, 1)
	go func() { closed <- server.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		_ = stream.Close()
		t.Fatal("server failed to interrupt and join blocked Application I/O")
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("server returned before Application Close")
	}
}

func TestSetupPeerLossCancelsOwnerAndJoinsServer(t *testing.T) {
	owner := &waitingOwner{entered: make(chan context.Context, 1)}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := connection.Dial(ctx, path, connection.Request{Destination: connection.TargetLink, Value: "explicit"})
		done <- err
	}()
	var requestContext context.Context
	select {
	case requestContext = <-owner.entered:
	case <-time.After(time.Second):
		t.Fatal("owner not entered")
	}
	cancel()
	select {
	case <-requestContext.Done():
	case <-time.After(time.Second):
		t.Fatal("peer loss did not cancel pending owner")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled setup accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("Dial failed to join")
	}
}

func TestServerRefusesCleanTerminalWhenApplicationCleanupFails(t *testing.T) {
	failure := errors.New("cleanup failed")
	stream := &waitingStream{closed: make(chan struct{}), done: make(chan connection.Outcome, 1), closeErr: failure}
	owner := &waitingOwner{entered: make(chan context.Context, 1), stream: stream}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := connection.Dial(t.Context(), path, connection.Request{Destination: connection.TargetLink, Value: "explicit"})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_ = stream.Close()
	if _, err := io.ReadAll(client); err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-client.Done():
		if outcome.Class != connection.IndeterminateFailure {
			t.Fatalf("failed cleanup terminal = %+v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup failure did not terminate")
	}
	if err := server.Close(); !errors.Is(err, failure) {
		t.Fatalf("lost cleanup cause: %v", err)
	}
}

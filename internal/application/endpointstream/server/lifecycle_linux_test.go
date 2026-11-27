//go:build linux

package server_test

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	endpointclient "github.com/dianabuilds/ardents-network/internal/application/endpointstream/client"
	endpointserver "github.com/dianabuilds/ardents-network/internal/application/endpointstream/server"
)

type waitingOwner struct {
	entered chan context.Context
	stream  *waitingStream
}

func (owner *waitingOwner) Open(ctx context.Context, _ endpointstream.Request) (endpointstream.Stream, error) {
	owner.entered <- ctx
	if owner.stream != nil {
		return owner.stream, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

type waitingStream struct {
	closed   chan struct{}
	done     chan endpointstream.Outcome
	once     sync.Once
	closeErr error
	terminal endpointstream.OutcomeClass
}

func (stream *waitingStream) Read([]byte) (int, error)            { <-stream.closed; return 0, io.EOF }
func (stream *waitingStream) Write([]byte) (int, error)           { <-stream.closed; return 0, net.ErrClosed }
func (stream *waitingStream) CloseInput() error                   { return nil }
func (stream *waitingStream) Done() <-chan endpointstream.Outcome { return stream.done }
func (stream *waitingStream) Close() error {
	stream.once.Do(func() {
		close(stream.closed)
		class := stream.terminal
		if class == "" {
			class = endpointstream.LocalCancellation
		}
		stream.done <- endpointstream.Outcome{Class: class}
		close(stream.done)
	})
	return stream.closeErr
}

func TestServerJoinsBlockedApplicationOnClose(t *testing.T) {
	stream := &waitingStream{closed: make(chan struct{}), done: make(chan endpointstream.Outcome, 1)}
	owner := &waitingOwner{entered: make(chan context.Context, 1), stream: stream}
	path := shortSocketPath(t)
	server, err := endpointserver.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := endpointclient.Dial(t.Context(), path, endpointstream.Request{Destination: endpointstream.TargetLink, Value: "explicit"})
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

func TestConnectionClosePreservesAReplacementListener(t *testing.T) {
	path := shortSocketPath(t)
	original, err := endpointserver.Listen(path, &waitingOwner{entered: make(chan context.Context, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	oldPath := filepath.Join(filepath.Dir(path), "old-"+filepath.Base(path))
	if err := os.Rename(path, oldPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(oldPath); err != nil {
			t.Error(err)
		}
	})

	outcome := endpointstream.Outcome{Class: endpointstream.ServiceUnavailable, Reason: "replacement listener"}
	replacement, err := endpointserver.Listen(path, setupRefusalOwner{outcome: outcome})
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	closeErr := original.Close()
	if closeErr == nil || original.Close() != closeErr {
		t.Fatal("original Connection did not retain its cleanup refusal", closeErr)
	}
	client, err := endpointclient.Dial(t.Context(), path, endpointstream.Request{Destination: endpointstream.TargetLink, Value: "explicit"})
	if client != nil {
		_ = client.Close()
		t.Fatal("replacement accepted an explicitly refused request")
	}
	var refusal endpointstream.SetupRefusalError
	if !errors.As(err, &refusal) || refusal.Outcome() != outcome {
		t.Fatal("original Connection cleanup interfered with the replacement", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatal("replacement could not retire its own original socket", err)
	}
}

func TestSetupPeerLossCancelsOwnerAndJoinsServer(t *testing.T) {
	owner := &waitingOwner{entered: make(chan context.Context, 1)}
	path := shortSocketPath(t)
	server, err := endpointserver.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := endpointclient.Dial(ctx, path, endpointstream.Request{Destination: endpointstream.TargetLink, Value: "explicit"})
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

type setupRefusalOwner struct{ outcome endpointstream.Outcome }

func (owner setupRefusalOwner) Open(context.Context, endpointstream.Request) (endpointstream.Stream, error) {
	return nil, endpointstream.Refuse(owner.outcome)
}

func TestDialPreservesRefusedSetupOutcome(t *testing.T) {
	path := shortSocketPath(t)
	server, err := endpointserver.Listen(path, setupRefusalOwner{outcome: endpointstream.Outcome{Class: endpointstream.ServiceUnavailable, Reason: "selected service is unavailable"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := server.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	client, err := endpointclient.Dial(t.Context(), path, endpointstream.Request{Destination: endpointstream.TargetLink, Value: "ardents-target:v1:unavailable"})
	if client != nil || err == nil {
		t.Fatalf("refused setup = client %v, error %v", client, err)
	}
	var refusal endpointstream.SetupRefusalError
	if !errors.As(err, &refusal) || refusal.Outcome().Class != endpointstream.ServiceUnavailable || refusal.Outcome().Reason != "selected service is unavailable" {
		t.Fatalf("refused setup lost its outcome: %v", err)
	}
}

func TestServerRefusesCleanTerminalWhenApplicationCleanupFails(t *testing.T) {
	failure := errors.New("cleanup failed")
	stream := &waitingStream{closed: make(chan struct{}), done: make(chan endpointstream.Outcome, 1), closeErr: failure, terminal: endpointstream.CleanClose}
	owner := &waitingOwner{entered: make(chan context.Context, 1), stream: stream}
	path := shortSocketPath(t)
	server, err := endpointserver.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client, err := endpointclient.Dial(t.Context(), path, endpointstream.Request{Destination: endpointstream.TargetLink, Value: "explicit"})
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
		if outcome.Class != endpointstream.IndeterminateFailure {
			t.Fatalf("failed cleanup terminal = %+v", outcome)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup failure did not terminate")
	}
	if err := server.Close(); !errors.Is(err, failure) {
		t.Fatalf("lost cleanup cause: %v", err)
	}
}

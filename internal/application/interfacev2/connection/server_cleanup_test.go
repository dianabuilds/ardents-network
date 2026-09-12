//go:build linux

package connection

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

type cleanupSequenceOwner struct {
	calls   atomic.Uint32
	failure error
}

func (owner *cleanupSequenceOwner) Open(context.Context, Request) (Stream, error) {
	var failure error
	if owner.calls.Add(1) == 1 {
		failure = owner.failure
	}
	done := make(chan Outcome, 1)
	done <- Outcome{Class: CleanClose}
	close(done)
	return &cleanupSequenceStream{done: done, failure: failure}, nil
}

type cleanupSequenceStream struct {
	done    <-chan Outcome
	failure error
}

func (*cleanupSequenceStream) Read([]byte) (int, error)       { return 0, io.EOF }
func (*cleanupSequenceStream) Write(body []byte) (int, error) { return len(body), nil }
func (*cleanupSequenceStream) CloseInput() error              { return nil }
func (stream *cleanupSequenceStream) Close() error            { return stream.failure }
func (stream *cleanupSequenceStream) Done() <-chan Outcome    { return stream.done }

func TestServerRetainsOneCleanupFailureAcrossClientTurnover(t *testing.T) {
	failure := errors.New("first cleanup failed")
	owner := &cleanupSequenceOwner{failure: failure}
	path := shortClientSocketPath(t)
	public, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	server := public.(*server)
	defer server.Close()
	for index := range 40 {
		client, err := Dial(t.Context(), path, Request{Destination: TargetLink, Value: "explicit"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(client); err != nil {
			t.Fatal(err)
		}
		outcome := <-client.Done()
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
		if index == 0 && outcome.Class != IndeterminateFailure {
			t.Fatal("failed cleanup reported clean")
		}
	}
	// Closing joins every server handler before reading the final state.
	if err := server.Close(); !errors.Is(err, failure) {
		t.Fatalf("lost cleanup failure: %v", err)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	// Listener/path cleanup may wrap once. It must not retain one node per
	// completed client, including successful clients after the first failure.
	nodes := 0
	var visit func(error)
	visit = func(err error) {
		if err == nil {
			return
		}
		nodes++
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				visit(child)
			}
		}
	}
	visit(server.err)
	if nodes > 4 {
		t.Fatalf("cleanup history grew with client turnover: %d nodes", nodes)
	}
}

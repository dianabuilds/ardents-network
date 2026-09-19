//go:build linux

package streamqualification

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

type bridgeTestStream struct {
	once    sync.Once
	closed  chan struct{}
	wrote   chan struct{}
	outcome chan connection.Outcome
	fault   error
}

func (s *bridgeTestStream) Read([]byte) (int, error) { <-s.closed; return 0, io.EOF }
func (s *bridgeTestStream) Write(b []byte) (int, error) {
	select {
	case s.wrote <- struct{}{}:
	default:
	}
	if s.fault != nil {
		return 0, s.fault
	}
	select {
	case <-s.closed:
		return 0, net.ErrClosed
	default:
		return len(b), nil
	}
}
func (s *bridgeTestStream) CloseInput() error               { return nil }
func (s *bridgeTestStream) Done() <-chan connection.Outcome { return s.outcome }
func (s *bridgeTestStream) Close() error {
	s.once.Do(func() {
		close(s.closed)
		s.outcome <- connection.Outcome{Class: connection.LocalCancellation}
		close(s.outcome)
	})
	return nil
}

func TestBridgeJoinsWorkerAndEveryRetainedStreamAfterCancellation(t *testing.T) {
	runBridgeRetirement(t, nil)
}
func TestBridgePreservesNetworkFailureAndJoinsSiblingStreams(t *testing.T) {
	runBridgeRetirement(t, errors.New("injected Service write failure"))
}
func runBridgeRetirement(t *testing.T, fault error) {
	t.Helper()
	ctx, stop := context.WithTimeout(t.Context(), 3*time.Second)
	defer stop()
	endpoint, worker := net.Pipe()
	init := Init{Role: ReaderRole, Profile: ClientToPublisher, Nonce: [32]byte{1}, Seed: [32]byte{2}}
	var streams []BoundStream
	wrote := make(chan struct{}, 1)
	for index := 0; index < 64; index++ {
		id := uint32(1 + 2*index)
		if index >= 16 {
			id = 129 + uint32(index-16)*2
		}
		stream := &bridgeTestStream{closed: make(chan struct{}), wrote: wrote, outcome: make(chan connection.Outcome, 1), fault: fault}
		streams = append(streams, BoundStream{ID: id, Stream: stream})
	}
	workerDone := make(chan error, 1)
	go func() { workerDone <- RunWorker(ctx, worker, init) }()
	bridgeDone := make(chan error, 1)
	go func() { _, err := RunConnections(ctx, endpoint, init, streams, nil); bridgeDone <- err }()
	select {
	case <-wrote:
	case <-ctx.Done():
		t.Fatal("concurrent bridge startup did not progress")
	}
	if fault == nil {
		stop()
	}
	select {
	case err := <-bridgeDone:
		if err == nil || fault != nil && !errors.Is(err, fault) {
			t.Fatalf("lost terminal cause: %v", err)
		}
	case <-ctx.Done():
		if fault != nil {
			t.Fatal("network failure did not retire siblings")
		}
		select {
		case <-bridgeDone:
		case <-time.After(time.Second):
			t.Fatal("cancel did not join bridge")
		}
	}
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("worker reader leaked")
	}
	for _, bound := range streams {
		select {
		case <-bound.Stream.(*bridgeTestStream).closed:
		default:
			t.Fatal("retained Service stream leaked")
		}
	}
}

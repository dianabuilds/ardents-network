package endpoint

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type applicationCloseFailure struct {
	*applicationHalfClose
	failure error
}

func (stream *applicationCloseFailure) Close() error {
	return errors.Join(stream.applicationHalfClose.Close(), stream.failure)
}

func TestApplicationConnectionCloseJoinsOwnerAndRetainsCleanupFailure(t *testing.T) {
	local, peer := newApplicationHalfClosePair()
	defer peer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	release := make(chan struct{})
	observedCancellation := make(chan struct{})
	failure := errors.New("fixture local cleanup failure")
	ownerFailure := errors.New("fixture owner cleanup failure")
	connection := &applicationConnection{stream: &applicationCloseFailure{applicationHalfClose: local, failure: failure}, cancel: cancel, finished: finished}
	go func() {
		<-ctx.Done()
		close(observedCancellation)
		<-release
		connection.finishErr = ownerFailure
		close(finished)
	}()
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	closed := make(chan error, 1)
	joined := make(chan struct{})
	go func() { defer close(joined); closed <- connection.Close() }()
	t.Cleanup(func() {
		cancel()
		unblock()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("Application close did not join")
		}
	})
	select {
	case <-observedCancellation:
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel its owner")
	}
	select {
	case <-closed:
		t.Fatal("Close returned while its owner still held work")
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	select {
	case err := <-closed:
		if !errors.Is(err, failure) || !errors.Is(err, ownerFailure) {
			t.Fatalf("cleanup errors lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not finish")
	}
	if err := connection.Close(); !errors.Is(err, failure) || !errors.Is(err, ownerFailure) {
		t.Fatal("repeated Close erased cleanup failure")
	}
}

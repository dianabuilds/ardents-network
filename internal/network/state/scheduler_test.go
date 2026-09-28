package state

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAutomaticRefreshRecordsTerminalFailure(t *testing.T) {
	owner := &networkState{}
	ticks := make(chan time.Time)
	owner.work.Add(1)
	go owner.runAutomaticRefresh(context.Background(), ticks, nil)
	ticks <- time.Now()
	owner.work.Wait()
	if owner.automaticErr == nil {
		t.Fatal("terminal automatic refresh failure was not retained")
	}
	if _, err := owner.Current(); err == nil {
		t.Fatal("Current accepted State after terminal automatic refresh failure")
	}
}

func TestWaitReportsTerminalAutomaticFailureAlongsideSourceServer(t *testing.T) {
	owner := &networkState{config: config{automatic: time.Second}, serverDone: make(chan struct{}), automaticDone: make(chan struct{})}
	ticks := make(chan time.Time)
	owner.work.Add(1)
	go func() {
		defer close(owner.automaticDone)
		owner.runAutomaticRefresh(context.Background(), ticks, nil)
	}()
	result := make(chan error, 1)
	go func() { result <- owner.Wait(context.Background()) }()
	ticks <- time.Now()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("Wait accepted terminal automatic refresh failure")
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not report terminal automatic refresh failure")
	}
	owner.work.Wait()
}

func TestWaitTracksResourceGovernorWithoutSourceWork(t *testing.T) {
	owner := &networkState{config: config{profile: "h3-s-v1"}, resourceDone: make(chan struct{})}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := owner.Wait(canceled); err != nil {
		t.Fatalf("Wait rejected resource-only background work: %v", err)
	}
	result := make(chan error, 1)
	go func() { result <- owner.Wait(context.Background()) }()
	failure := errors.New("resource governor failed")
	owner.mu.Lock()
	owner.resourceErr = failure
	owner.mu.Unlock()
	close(owner.resourceDone)
	select {
	case err := <-result:
		if !errors.Is(err, failure) {
			t.Fatalf("Wait returned %v, want resource failure", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not report resource governor failure")
	}
}

func TestWaitDoesNotMaskResourceFailureWithCanceledSource(t *testing.T) {
	failure := errors.New("resource governor failed")
	serverDone, resourceDone := make(chan struct{}), make(chan struct{})
	close(serverDone)
	close(resourceDone)
	owner := &networkState{serverDone: serverDone, resourceDone: resourceDone,
		serverErr: context.Canceled, resourceErr: failure}
	if err := owner.Wait(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Wait returned %v, want resource failure", err)
	}
}

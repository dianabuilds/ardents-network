package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCloseRetainsTerminalAndSourceReleaseFailures(t *testing.T) {
	for _, test := range []struct {
		name         string
		serverErr    error
		automaticErr error
		terminalErr  error
	}{
		{name: "server failure", serverErr: errors.New("source server failed")},
		{name: "automatic refresh failure after source cancellation", serverErr: context.Canceled, automaticErr: errors.New("automatic refresh failed")},
		{name: "terminal failure after source cancellation", serverErr: context.Canceled, terminalErr: errors.New("Network owner failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			storage, err := openTestDurableRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			state := &networkState{storage: storage, serverErr: test.serverErr, automaticErr: test.automaticErr, terminalErr: test.terminalErr}
			state.config.sourceInfo.Serving = true
			state.config.localRoles = filepath.Join(t.TempDir(), "absent-local-role-root")
			state.config.clock = time.Now
			releaseErr := state.releaseSourceServer()
			if releaseErr == nil {
				t.Fatal("missing local-role root did not fail")
			}
			got := state.Close()
			if got == nil || !strings.Contains(got.Error(), releaseErr.Error()) {
				t.Fatalf("Close() = %v, missing source-role release failure %v", got, releaseErr)
			}
			primary := test.serverErr
			if test.automaticErr != nil {
				primary = test.automaticErr
			}
			if test.terminalErr != nil {
				primary = test.terminalErr
			}
			if !errors.Is(got, primary) {
				t.Fatalf("Close() = %v, missing terminal failure %v", got, primary)
			}
			again := state.Close()
			if !errors.Is(again, primary) || !strings.Contains(again.Error(), releaseErr.Error()) {
				t.Fatalf("second Close() = %v, want the same terminal and release failures", again)
			}
			if reopened, err := openTestDurableRoot(root); err != nil {
				t.Fatalf("State root remained locked after Close: %v", err)
			} else if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCloseTreatsSourceCancellationAsExpected(t *testing.T) {
	storage, err := openTestDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state := &networkState{storage: storage, serverErr: context.Canceled}
	if err := state.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil for expected Source cancellation", err)
	}
}

func TestConcurrentCloseWaitsForCleanupAndRetainsFailure(t *testing.T) {
	storage, err := openTestDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	terminalErr := errors.New("source server failed")
	canceled := make(chan struct{})
	state := &networkState{
		storage:    storage,
		serverErr:  terminalErr,
		workCancel: func() { close(canceled) },
	}
	state.work.Add(1)
	first := make(chan error, 1)
	go func() { first <- state.Close() }()
	<-canceled

	second := make(chan error, 1)
	go func() { second <- state.Close() }()
	var premature error
	prematureReturn := false
	select {
	case premature = <-second:
		prematureReturn = true
	case <-time.After(50 * time.Millisecond):
	}
	state.work.Done()
	firstErr := <-first
	secondErr := premature
	if !prematureReturn {
		secondErr = <-second
	}
	if !errors.Is(firstErr, terminalErr) {
		t.Fatalf("first Close() = %v, want %v", firstErr, terminalErr)
	}
	if prematureReturn {
		t.Fatalf("concurrent Close() returned before cleanup finished: %v", premature)
	}
	if !errors.Is(secondErr, terminalErr) {
		t.Fatalf("second Close() = %v, want %v", secondErr, terminalErr)
	}
}

func TestCloseDoesNotTurnScheduledRefreshIntoTerminalFailure(t *testing.T) {
	storage, err := openTestDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ticks := make(chan time.Time)
	joined := make(chan struct{})
	owner := &networkState{storage: storage}
	owner.work.Add(1)
	go func() {
		defer close(joined)
		owner.runAutomaticRefresh(ctx, ticks, nil)
	}()
	// Close has already refused new work when cancellation is invoked.
	// Deliver the selected tick in that exact interval, before cancel reaches
	// the scheduler, rather than depending on wall-clock timing.
	owner.workCancel = func() {
		select {
		case ticks <- time.Now():
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatal("scheduler did not receive the closing tick")
		}
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatal("scheduler did not retire after the closing tick")
		}
		cancel()
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("orderly close reported scheduler failure: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("repeated close: %v", err)
	}
}

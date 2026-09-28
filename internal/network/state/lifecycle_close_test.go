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
		name        string
		serverErr   error
		resourceErr error
	}{
		{name: "server failure", serverErr: errors.New("source server failed")},
		{name: "resource failure after source cancellation", serverErr: context.Canceled, resourceErr: errors.New("resource accounting failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			storage, err := openTestDurableRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			state := &networkState{storage: storage, serverErr: test.serverErr, resourceErr: test.resourceErr}
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
			if test.resourceErr != nil {
				primary = test.resourceErr
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

//go:build linux || darwin

package node

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// This isolates terminal-handle provenance, not successful authority or native
// physical join. The genuine native command matrix owns those separate claims.
func TestCopiedForwardingHandleRetainsLateOriginalCleanupFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &forwarding{nodeLifetime: nodeLifetime{ctx: ctx, cancel: cancel, done: make(chan struct{})}}
	handle := &Forwarding{original: original}
	copy := *handle
	cause := errors.New("original physical cleanup failed")
	original.first, original.cleanupResult, original.result = context.Canceled, cause, cause
	close(original.done)
	if copy.Done() != handle.Done() || copy.Close() != cause || handle.Close() != cause || copy.JoinedOutcome() != "failed" {
		t.Fatal("copied handle reported another terminal result")
	}
	var absent Forwarding
	if absent.Done() != nil || absent.Close() == nil || absent.JoinedOutcome() != "failed" {
		t.Fatal("zero handle fabricated original completion")
	}
}

func TestOriginalObservationCancellationDoesNotMintCleanupFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	notice := errors.New("private original retirement")
	f := &forwarding{nodeLifetime: nodeLifetime{ctx: ctx, cancel: cancel, retired: notice}}
	if got := f.failedObservation(context.Canceled, "process"); got != notice || f.first != context.Canceled {
		t.Fatal("original bare caller cancellation became a foreign cleanup cause")
	}
	for _, cause := range []error{fmt.Errorf("physical close: %w", context.Canceled), errors.Join(context.Canceled, errors.New("close failed")), errors.New(context.Canceled.Error())} {
		f := &forwarding{nodeLifetime: nodeLifetime{ctx: ctx, cancel: cancel, retired: notice}}
		if got := f.failedObservation(cause, "process"); got != cause || f.first != cause {
			t.Fatal("genuine foreign or physical failure replaced", cause)
		}
	}
}

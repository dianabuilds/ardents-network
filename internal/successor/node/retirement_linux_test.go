//go:build linux

package node

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestOriginalRetirementNoticeCannotErasePhysicalCleanupFailure(t *testing.T) {
	notice := errors.New("original local admission retired")
	foreign := errors.New("physical close failed")
	if originalRetirementResult(errors.Join(notice, errors.Join(notice)), notice) != nil {
		t.Fatal("local denial notice reported as physical cleanup failure")
	}
	for _, physical := range []error{foreign, context.Canceled, fmt.Errorf("physical join: %w", context.Canceled), fmt.Errorf("physical join: %w", notice), errors.New(notice.Error())} {
		retained := originalRetirementResult(errors.Join(notice, physical), notice)
		if !errors.Is(retained, physical) {
			t.Fatal("actual foreign or wrapped physical result suppressed", physical)
		}
	}
}

// This isolates terminal-handle provenance, not successful authority or native
// physical join. The genuine native command matrix owns those separate claims.
func TestCopiedForwardingHandleRetainsLateOriginalCleanupFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &forwarding{ctx: ctx, cancel: cancel, done: make(chan struct{})}
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

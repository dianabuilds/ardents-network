package connection

import (
	"context"
	"errors"
	"testing"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

func TestDetachedInitialStateCannotExposeApplicationStream(t *testing.T) {
	for _, owner := range []*AuthenticatedAttachment{nil, {}} {
		if stream, err := owner.OpenStream(executionruntime.StreamContract{}); err == nil || stream != nil {
			t.Fatal("detached initial state supplied an Application stream")
		}
	}
}

// This retirement fixture has no successful Instance, Operation or transport.
func TestCopiedStreamCannotInterruptOriginalLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &Stream{ctx: ctx, cancel: cancel}
	original.self = original
	copy := &Stream{self: original, ctx: ctx, cancel: cancel}
	if err := copy.Close(); !errors.Is(err, errIntegrity) || ctx.Err() != nil {
		t.Fatal("copied stream interrupted its original", err, ctx.Err())
	}
}

// A completed exchange of Terminal records still requires the original
// operation before the Stream publishes an Application outcome.
func TestTerminalFlagsCannotPublishSuccessWithoutOriginalOperation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &Stream{ctx: ctx, cancel: cancel, owner: &AuthenticatedAttachment{},
		transfer: completedTransfer(t), outcome: make(chan application.Outcome, 1), settledDone: make(chan struct{})}
	stream.mu.Lock()
	stream.settleLocked()
	stream.mu.Unlock()
	if stream.settled || stream.failure == nil || ctx.Err() == nil {
		t.Fatal("synthetic Terminal flags bypassed original authority")
	}
	select {
	case <-stream.outcome:
		t.Fatal("missing Operation published successful terminal outcome")
	default:
	}
}

package runtime

import (
	"context"
	"testing"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

// This fixture supplies no accepting authority. It tests refusal before a
// copied handle can cancel original users or start a physical cleanup wait.
func TestCopiedPublisherCannotRetireOrAcquireOriginal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	original := &Publisher{ctx: ctx, cancel: cancel}
	original.self = original
	copied := &Publisher{self: original, ctx: ctx, cancel: cancel}
	if source, err := copied.CurrentSource(t.Context()); source != nil || err == nil {
		t.Fatal("copied Publisher acquired the original Source", err)
	}
	copied.cleanupJoined.Store(true)
	copied.watcherDone = make(chan struct{})
	close(copied.watcherDone)
	if _, joined := copied.CleanupResult(); joined {
		t.Fatal("copied local completion supplied an original cleanup receipt")
	}
	if err := copied.Close(); err == nil {
		t.Fatal("copied Publisher entered original physical retirement")
	}
	if err := copied.Withdraw(t.Context()); err == nil {
		t.Fatal("copied Publisher withdrew original Publication")
	}
	if _, err := copied.NextStream(t.Context(), executionruntime.StreamContract{}); err == nil {
		t.Fatal("copy acquired an original stream")
	}
	if _, err := copied.Link(t.Context()); err == nil {
		t.Fatal("copy returned original accepting Link")
	}
	if _, err := copied.Publish(t.Context()); err == nil {
		t.Fatal("copy admitted original publication effects")
	}
	if ctx.Err() != nil || original.closed || original.flight || copied.closed || copied.flight {
		t.Fatal("copy changed original lifetime or admitted work", ctx.Err())
	}
}

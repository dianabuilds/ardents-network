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

// This isolates cancellation ownership; it supplies no registration, ACK or
// accepting Publication. The installed scenario checks the real handoff.
func TestRegistrationSetupCallerTransfer(t *testing.T) {
	for _, canceledBefore := range []bool{false, true} {
		t.Run(map[bool]string{false: "after-transfer", true: "before-transfer"}[canceledBefore], func(t *testing.T) {
			parent, stopParent := context.WithCancel(t.Context())
			defer stopParent()
			caller, stopCaller := context.WithCancel(t.Context())
			defer stopCaller()
			child, stopChild := context.WithCancel(parent)
			defer stopChild()
			pair := &registrationPair{ctx: child, cancel: stopChild, callerCallback: make(chan struct{})}
			pair.stopCaller = context.AfterFunc(caller, func() { defer close(pair.callerCallback); stopChild() })
			if canceledBefore {
				stopCaller()
			}
			pair.joinCaller()
			stopCaller()
			pair.joinCaller() // Later joined cleanup must not wait on a stopped callback.
			if canceledBefore && child.Err() == nil {
				t.Fatal("winning setup cancellation was lost")
			}
			if !canceledBefore && child.Err() != nil {
				t.Fatal("completed setup caller canceled retained pair")
			}
			stopParent()
			if child.Err() == nil {
				t.Fatal("transfer detached original Publisher lifetime")
			}
		})
	}
}

package connection

import (
	"context"
	"errors"
	"testing"
)

// This isolates callback retirement and cancellation, without constructing an
// accepting Execution, Instance or Route fixture or claiming Service transfer.
func TestPublisherCommitRetainsCallerAfterSetupCallbackJoin(t *testing.T) {
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	child, cancelChild := context.WithCancel(context.Background())
	defer cancelChild()
	proposal := &PublisherReplacement{stream: &Stream{}, caller: caller, ctx: child,
		callerDone: make(chan struct{}), interruptionDone: make(chan struct{})}
	proposal.stopCaller = context.AfterFunc(caller, func() {
		defer close(proposal.callerDone)
		cancelChild()
	})
	proposal.stopInterruption = context.AfterFunc(child, func() {
		close(proposal.interruptionDone)
	})
	proposal.joinSetup()
	cancelCaller()
	if child.Err() != nil {
		t.Fatal("retired setup callback interrupted the independently retained child")
	}
	if err := proposal.checkCommitStateLocked(); !errors.Is(err, context.Canceled) {
		t.Fatalf("detached callback erased original caller cancellation: %v", err)
	}
}

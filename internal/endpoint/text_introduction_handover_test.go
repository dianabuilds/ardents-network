//go:build linux

package endpoint

import (
	"context"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Cancellation is observable before the AfterFunc callback is scheduled.
// Stopping the registration removes the queued callback, as a context permits.
type textDelayedCancellation struct {
	context.Context
	mu        sync.Mutex
	done      chan struct{}
	cancelled bool
	callback  func()
}

func (ctx *textDelayedCancellation) Done() <-chan struct{} { return ctx.done }
func (ctx *textDelayedCancellation) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.cancelled {
		return context.Canceled
	}
	return nil
}
func (ctx *textDelayedCancellation) AfterFunc(callback func()) func() bool {
	ctx.mu.Lock()
	ctx.callback = callback
	ctx.mu.Unlock()
	return func() bool {
		ctx.mu.Lock()
		defer ctx.mu.Unlock()
		pending := ctx.callback != nil
		ctx.callback = nil
		return pending
	}
}
func (ctx *textDelayedCancellation) cancelBeforeCallback() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if !ctx.cancelled {
		ctx.cancelled = true
		close(ctx.done)
	}
}

type textPreparationBoundaryState struct {
	*textSourceStateFixture
	reads    int
	cancelAt int
	caller   *textDelayedCancellation
}

func (source *textPreparationBoundaryState) CurrentClosedProfile() (state.ClosedProfileView, error) {
	source.reads++
	if source.reads == source.cancelAt {
		source.caller.cancelBeforeCallback()
	}
	return source.textSourceStateFixture.CurrentClosedProfile()
}

func checkTextPreparationCallerHandover(t *testing.T, owner *textContext, job *textJobIdentity, source *textSourceStateFixture,
	destination targetlink.Link, bounds [3]int64) {
	t.Helper()
	endpoint := owner.endpoint
	counting := &textPreparationBoundaryState{textSourceStateFixture: source}
	endpoint.closedState = counting
	defer func() { endpoint.closedState = source }()
	// Measure the last current-profile read in an otherwise identical warm
	// attempt; neither pass refills the already populated token stock.
	prepared, err := owner.prepareTextIntroduction(t.Context(), job, destination, bounds)
	if err != nil {
		t.Fatal(err)
	}
	clear(prepared.operation)
	if counting.reads == 0 {
		t.Fatal("preparation did not verify State")
	}
	caller := &textDelayedCancellation{Context: context.Background(), done: make(chan struct{})}
	paused := &textPreparationBoundaryState{textSourceStateFixture: source, cancelAt: counting.reads, caller: caller}
	endpoint.closedState = paused
	prepared, err = owner.prepareTextIntroduction(caller, job, destination, bounds)
	if paused.reads != paused.cancelAt || caller.Err() == nil {
		t.Fatal("test did not cancel at the final authority read")
	}
	if err == nil || prepared != nil {
		t.Errorf("cancelled caller received a prepared capsule: %v", err)
	}
	caller.mu.Lock()
	pending := caller.callback != nil
	caller.mu.Unlock()
	if pending {
		t.Error("preparation retained cancellation callback")
	}
}

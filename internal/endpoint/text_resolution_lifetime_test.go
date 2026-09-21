//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// Pause only the public State read before recipient selection. The test proves
// the real lookup flight is cancelled and joined, not an installed-host claim.
type textPausedResolutionState struct {
	active atomic.Bool
	*textSourceStateFixture
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (source *textPausedResolutionState) CurrentClosedRoute() (state.ClosedRouteView, error) {
	if source.active.Load() {
		source.once.Do(func() { close(source.entered); <-source.release })
	}
	return source.textSourceStateFixture.CurrentClosedRoute()
}

func TestTextResolutionCloseJoinsInFlightStateSelection(t *testing.T) {
	endpoint, owner, source := startTextControlNetwork(t, route.ClosedCarrierTCP, true)
	paused := &textPausedResolutionState{textSourceStateFixture: source, entered: make(chan struct{}), release: make(chan struct{})}
	endpoint.closedState = paused
	prefix, err := owner.openTextPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// The opened prefix retains this wrapper, while Node runtimes use the
	// original source. The first lookup State read now pauses outside owner.mu.
	paused.active.Store(true)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(paused.release) }) }
	defer release()
	result := make(chan error, 1)
	go func() { _, err := owner.lookupTextDescriptor(t.Context(), fixtureID(199)); result <- err }()
	select {
	case <-paused.entered:
	case err := <-result:
		t.Fatalf("resolution ended before selected boundary: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("resolution did not reach State selection")
	}
	owner.mu.Lock()
	acquisition, acquired := owner.resolution.source.(*textSourceResolutionAcquisition)
	owner.mu.Unlock()
	if !acquired || acquisition == nil {
		t.Fatal("lookup did not own an exact Source acquisition")
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	select {
	case <-prefix.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not retire prefix")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned before the resolution flight joined: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	release()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("revoked lookup succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("resolution did not join")
	}

	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("context retained resolution")
	}
	if acquisition.handle.Load() != nil {
		t.Fatal("cancelled lookup retained its Source acquisition")
	}
	select {
	case <-prefix.Done():
	default:
		t.Fatal("revocation left prefix alive")
	}
	if _, err := owner.lookupTextDescriptor(context.Background(), fixtureID(199)); err == nil {
		t.Fatal("closed context admitted another lookup")
	}
}

func TestTextResolutionCompletionRetainsFailedCleanup(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	attempt, cancel := context.WithCancel(owner.lease.Context())
	defer cancel()
	flight := &textResolutionFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	owner.mu.Lock()
	owner.resolution = flight
	owner.mu.Unlock()
	original := errors.New("terminal CLOSE could not be emitted")
	failure := errors.Join(route.ErrClosedSourceCleanup, original)
	owner.finishTextResolution(flight, failure)
	if endpoint.textAvailable() {
		t.Fatal("cleanup failure left Endpoint accepting jobs")
	}
	if err := owner.Close(); !errors.Is(err, original) {
		t.Fatalf("context lost original cleanup error: %v", err)
	}
	if err := owner.Close(); !errors.Is(err, route.ErrClosedSourceCleanup) {
		t.Fatalf("repeated close lost cleanup class: %v", err)
	}
	if err := endpoint.Close(); !errors.Is(err, original) {
		t.Fatalf("Endpoint lost failed resolution: %v", err)
	}
}

func TestTextResolutionOldAcquisitionCannotCommitAfterSourceReplacement(t *testing.T) {
	endpoint, owner, source := startTextControlNetwork(t, route.ClosedCarrierTCP, true)
	defer func() { _ = endpoint.Close() }()
	old, err := owner.openTextPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	target, raw := textResolutionProof(t, source)
	defer clear(raw)

	owner.mu.Lock()
	profile, _, err := owner.textPermissionProfileLocked()
	if err != nil {
		owner.mu.Unlock()
		t.Fatal(err)
	}
	acquisition := owner.source.acquireResolutionLocked()
	if acquisition == nil {
		owner.mu.Unlock()
		t.Fatal("resolution acquisition unavailable")
	}
	attempt, cancel := context.WithCancel(owner.lease.Context())
	flight := &textResolutionFlight{context: attempt, cancel: cancel, done: make(chan struct{}), source: acquisition, releaseSource: acquisition.release}
	owner.resolution = flight
	owner.mu.Unlock()
	finished := false
	defer func() {
		if !finished {
			cancel()
			owner.finishTextResolution(flight, context.Canceled)
		}
	}()
	if err := owner.prepareTextSourceReopen(t.Context(), flight); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	err = owner.retireTextPrefixLocked()
	owner.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := owner.openTextPrefix(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	verified, commitErr := owner.acceptTextResolutionResult(t.Context(), flight, profile, target, raw)
	owner.mu.Lock()
	_, committed := owner.descriptorFloors[target]
	retained := owner.currentTextSourceLocked() == replacement && owner.resolution == flight
	owner.mu.Unlock()
	if commitErr == nil || verified.Descriptor.Target != [32]byte{} || committed || !retained {
		t.Fatalf("old acquisition committed after replacement: err=%v committed=%v retained=%v", commitErr, committed, retained)
	}
	cancel()
	owner.finishTextResolution(flight, commitErr)
	finished = true
	if acquisition.handle.Load() != nil {
		t.Fatal("resolution completion retained its acquisition")
	}
}

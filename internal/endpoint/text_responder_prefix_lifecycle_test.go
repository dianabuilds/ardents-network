//go:build linux

package endpoint

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (handle *textResponderPrefixHandle) Close() error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Close()
}

func (handle *textResponderPrefixHandle) Done() <-chan struct{} {
	prefix, err := handle.routePrefix()
	if err != nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return prefix.Done()
}

func TestTextResponderCancelledOpeningDoesNotRetireIntroductionOrPublishPrefix(t *testing.T) {
	owner := &textContext{}
	introduction := &textIntroductionPrefixHandle{owner: &owner.introduction, cancel: func() {}}
	introduction.prefix.Store(&route.ClosedSourcePrefix{})
	owner.introduction.live = introduction
	attempt, cancel := context.WithCancel(t.Context())
	flight := &textSourceFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	if !owner.responder.reserveOpeningLocked(flight) {
		t.Fatal("Responder lifecycle refused its first opening")
	}
	retirement := owner.responder.stopLocked()
	if attempt.Err() == nil {
		t.Fatal("Responder stop did not cancel its opening")
	}
	late := &route.ClosedSourcePrefix{}
	if owner.responder.finishOpeningLocked(flight, late, func() {}, true) {
		t.Fatal("cancelled Responder opening published a usable prefix")
	}
	close(flight.done)
	retirement.joinOpening()
	if err := retirement.closePrefix(); err != nil {
		t.Fatal(err)
	}
	if owner.introduction.live != introduction || introduction.prefix.Load() == nil {
		t.Fatal("Responder cancellation retired its sibling Introduction")
	}
	if owner.responder.currentLocked() != nil || owner.responder.opening != nil {
		t.Fatal("cancelled Responder opening remained usable")
	}
}

func TestTextResponderOldOpeningCannotAcquireReplacementHandle(t *testing.T) {
	var lifecycle textResponderPrefixLifecycle
	firstFlight := &textSourceFlight{}
	if !lifecycle.reserveOpeningLocked(firstFlight) {
		t.Fatal("Responder lifecycle refused first opening")
	}
	first := &route.ClosedSourcePrefix{}
	if !lifecycle.finishOpeningLocked(firstFlight, first, func() {}, true) {
		t.Fatal("Responder lifecycle refused first completion")
	}
	retirement := lifecycle.stopLocked()
	secondFlight := &textSourceFlight{}
	if !lifecycle.reserveOpeningLocked(secondFlight) {
		t.Fatal("Responder lifecycle refused replacement opening")
	}
	second := &route.ClosedSourcePrefix{}
	if !lifecycle.finishOpeningLocked(secondFlight, second, func() {}, true) {
		t.Fatal("Responder lifecycle refused replacement completion")
	}
	if lifecycle.acquireOpenedLocked(first) != nil {
		t.Fatal("old Responder opening acquired replacement handle")
	}
	if handle := lifecycle.acquireOpenedLocked(second); handle == nil || handle.prefix.Load() != second {
		t.Fatal("replacement opening did not retain its exact handle")
	}
	retirement.prefix = nil // Zero-value Route prefixes are identity fixtures only.
}

//go:build linux

package endpoint

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route/client"
)

func (handle *responderPrefixHandle) Close() error {
	prefix, err := handle.routePrefix()
	if err != nil {
		return err
	}
	return prefix.Close()
}

func (handle *responderPrefixHandle) Done() <-chan struct{} {
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
	introduction := &introductionPrefixHandle{rolePrefixHandleCore: rolePrefixHandleCore{owner: &owner.introduction.prefix.rolePrefixCore, cancel: func() {}}}
	introduction.prefix.Store(&client.ClosedSourcePrefix{})
	owner.introduction.prefix.live = introduction
	attempt, cancel := context.WithCancel(t.Context())
	flight := &textOperationFlight{context: attempt, cancelOperation: cancel, done: make(chan struct{})}
	if !owner.responder.reserveOpeningLocked(flight) {
		t.Fatal("Responder lifecycle refused its first opening")
	}
	retirement := owner.responder.stopLocked()
	if attempt.Err() == nil {
		t.Fatal("Responder stop did not cancel its opening")
	}
	late := &client.ClosedSourcePrefix{}
	if owner.responder.finishOpeningLocked(flight, late, func() {}, true) {
		t.Fatal("cancelled Responder opening published a usable prefix")
	}
	close(flight.done)
	retirement.joinOpening()
	if err := retirement.closePrefix(); err != nil {
		t.Fatal(err)
	}
	if owner.introduction.prefix.live != introduction || introduction.prefix.Load() == nil {
		t.Fatal("Responder cancellation retired its sibling Introduction")
	}
	if owner.responder.currentLocked() != nil || owner.responder.opening != nil {
		t.Fatal("cancelled Responder opening remained usable")
	}
}

func TestTextResponderOldOpeningCannotAcquireReplacementHandle(t *testing.T) {
	var lifecycle responderPrefixLifecycle
	firstFlight := &textOperationFlight{}
	if !lifecycle.reserveOpeningLocked(firstFlight) {
		t.Fatal("Responder lifecycle refused first opening")
	}
	first := &client.ClosedSourcePrefix{}
	if !lifecycle.finishOpeningLocked(firstFlight, first, func() {}, true) {
		t.Fatal("Responder lifecycle refused first completion")
	}
	retirement := lifecycle.stopLocked()
	secondFlight := &textOperationFlight{}
	if !lifecycle.reserveOpeningLocked(secondFlight) {
		t.Fatal("Responder lifecycle refused replacement opening")
	}
	second := &client.ClosedSourcePrefix{}
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

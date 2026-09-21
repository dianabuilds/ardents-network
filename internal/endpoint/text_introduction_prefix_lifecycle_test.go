//go:build linux

package endpoint

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (handle *textIntroductionPrefixHandle) Done() <-chan struct{} {
	prefix, err := handle.routePrefix()
	if err != nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	return prefix.Done()
}

func TestTextIntroductionCancelledOpeningDoesNotRetireSourceOrPublishPrefix(t *testing.T) {
	owner := &textContext{}
	source := &textSourceHandle{owner: &owner.source, cancel: func() {}}
	source.prefix.Store(&route.ClosedSourcePrefix{})
	owner.source.live = source
	attempt, cancel := context.WithCancel(t.Context())
	flight := &textSourceFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	if !owner.introduction.reserveOpeningLocked(flight) {
		t.Fatal("Introduction lifecycle refused its first opening")
	}
	retirement := owner.introduction.stopLocked()
	if attempt.Err() == nil {
		t.Fatal("Introduction stop did not cancel its opening")
	}
	late := &route.ClosedSourcePrefix{}
	if owner.introduction.finishOpeningLocked(flight, late, func() {}, true) {
		t.Fatal("cancelled Introduction opening published a usable prefix")
	}
	close(flight.done)
	retirement.joinOpening()
	if err := retirement.closePrefix(); err != nil {
		t.Fatal(err)
	}
	if owner.source.live != source || source.prefix.Load() == nil {
		t.Fatal("Introduction cancellation retired its borrowed Source")
	}
	if owner.introduction.currentLocked() != nil || owner.introduction.openingInProgressLocked() {
		t.Fatal("cancelled Introduction opening remained usable")
	}
}

func TestTextIntroductionOldOpeningCannotAcquireReplacementHandle(t *testing.T) {
	var lifecycle textIntroductionPrefixLifecycle
	firstFlight := &textSourceFlight{}
	if !lifecycle.reserveOpeningLocked(firstFlight) {
		t.Fatal("Introduction lifecycle refused first opening")
	}
	first := &route.ClosedSourcePrefix{}
	if !lifecycle.finishOpeningLocked(firstFlight, first, func() {}, true) {
		t.Fatal("Introduction lifecycle refused first completion")
	}
	retirement := lifecycle.stopLocked()
	secondFlight := &textSourceFlight{}
	if !lifecycle.reserveOpeningLocked(secondFlight) {
		t.Fatal("Introduction lifecycle refused replacement opening")
	}
	second := &route.ClosedSourcePrefix{}
	if !lifecycle.finishOpeningLocked(secondFlight, second, func() {}, true) {
		t.Fatal("Introduction lifecycle refused replacement completion")
	}
	if lifecycle.acquireOpenedLocked(first) != nil {
		t.Fatal("old Introduction opening acquired replacement handle")
	}
	if handle := lifecycle.acquireOpenedLocked(second); handle == nil || handle.prefix.Load() != second {
		t.Fatal("replacement opening did not retain its exact handle")
	}
	retirement.prefix = nil // Zero-value Route prefixes are identity fixtures only.
}

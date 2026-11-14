//go:build linux

package publisher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	publicationruntime "github.com/dianabuilds/ardents-network/internal/successor/publication/runtime"
)

// These controls isolate composition retirement, not successful Publication,
// worker qualification, Instance authority or an installed journey.
func TestRetirementWaitsForOriginalPhysicalCloseAndRetainsFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		failure := errors.New("original physical cleanup failed")
		entered, release := make(chan struct{}), make(chan struct{})
		var closes atomic.Int32
		publication := &published{cancel: cancel, handle: Publication{owner: &publicationruntime.Publisher{}, close: func() error {
			closes.Add(1)
			close(entered)
			<-release
			return failure
		}}}
		done := make(chan error, 2)
		go func() { done <- publication.Close() }()
		<-entered
		if ctx.Err() == nil || !publication.retiring.Load() {
			t.Fatal("physical join started before retirement")
		}
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatal("Close returned before physical join", err)
		default:
		}
		go func() { done <- publication.Close() }()
		close(release)
		for range 2 {
			if err := <-done; !errors.Is(err, failure) {
				t.Fatal("original cleanup failure lost", err)
			}
		}
		if closes.Load() != 1 {
			t.Fatal("physical owner closed more than once", closes.Load())
		}
	})
}

func TestPublisherWithoutQualifiedOwnersRefusesBeforeSnapshotEffects(t *testing.T) {
	owner := New(Config{Context: t.Context()})
	body := []byte("original snapshot")
	if err := owner.PublishSnapshot(t.Context(), body); err == nil {
		t.Fatal("unqualified snapshot accepted")
	}
	if string(body) != "original snapshot" || owner.active != nil || owner.busy {
		t.Fatal("refusal changed caller bytes or publication state")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

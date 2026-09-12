//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestTextIntroductionExchangeCloseJoinsAndRetainsCleanupFailure(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job := liveTextCapsuleJob(t, owner)
	lifetime, finish, err := owner.beginTextIntroductionExchange(t.Context(), job, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			finish(nil)
		}
	}()
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	select {
	case <-lifetime.Done():
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel exchange")
	}
	// Worker cleanup alone does not release the still-owned exchange.
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
		t.Fatal("Close returned before exchange joined")
	case <-time.After(25 * time.Millisecond):
	}
	original := errors.New("delivery child cleanup failed")
	outcome := finish(errors.Join(route.ErrClosedSourceCleanup, original))
	finished = true
	if !errors.Is(outcome, original) || !errors.Is(outcome, context.Canceled) {
		t.Fatal("exchange lost original cleanup/cancellation")
	}
	select {
	case err := <-closed:
		if !errors.Is(err, original) {
			t.Fatalf("Close lost failure: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not join completed exchange")
	}
	if endpoint.textAvailable() {
		t.Fatal("failed cleanup left new jobs enabled")
	}
	if err := owner.Close(); !errors.Is(err, original) {
		t.Fatal("repeated Close lost failure")
	}
	if err := endpoint.Close(); !errors.Is(err, original) {
		t.Fatalf("Endpoint Close lost failure: %v", err)
	}
}

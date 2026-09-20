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

func TestTextServiceTransportExchangeRetainsCleanupAfterJobLoss(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job := liveTextCapsuleJob(t, owner)
	lifetime, flight, detach, finish, err := owner.beginTextServiceTransportExchange(t.Context(), job, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			finish(nil)
		}
	}()
	if !detach() || !owner.retainTextServiceTransportExchange(job, flight) {
		t.Fatal("accepted transport did not transfer its cleanup lifetime")
	}
	closed := make(chan error, 1)
	go func() { closed <- owner.Close() }()
	select {
	case <-job.context.Done():
	case <-time.After(time.Second):
		t.Fatal("Close did not revoke job authority")
	}
	deadline := time.Now().Add(time.Second)
	for {
		owner.mu.Lock()
		retired := job.retired
		owner.mu.Unlock()
		if retired {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Close did not retire job ownership")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-lifetime.Done():
		t.Fatal("job revocation interrupted retained transport cleanup")
	default:
	}
	if outcome := finish(nil); !errors.Is(outcome, context.Canceled) {
		t.Fatalf("transport cleanup lost revoked job outcome: %v", outcome)
	}
	finished = true
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not join retained transport cleanup")
	}
}

func TestTextServiceTransportExchangeIgnoresDetachedCallerCancellation(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	job := liveTextCapsuleJob(t, owner)
	caller, cancel := context.WithCancel(t.Context())
	lifetime, flight, detach, finish, err := owner.beginTextServiceTransportExchange(caller, job, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	if !detach() || !owner.retainTextServiceTransportExchange(job, flight) {
		t.Fatal("accepted transport did not transfer its cleanup lifetime")
	}
	cancel()
	select {
	case <-lifetime.Done():
		t.Fatal("detached caller interrupted retained transport cleanup")
	default:
	}
	if err := finish(nil); err != nil {
		t.Fatalf("detached caller changed transport cleanup: %v", err)
	}
}

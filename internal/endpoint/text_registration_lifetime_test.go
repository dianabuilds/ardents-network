//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestTextRegistrationCompletionRetainsFailedCleanup(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Administration)
	attempt, cancel := context.WithCancel(owner.lease.Context())
	defer cancel()
	flight := &textRegistrationFlight{context: attempt, cancel: cancel, done: make(chan struct{})}
	owner.registrationOpening = flight
	original := errors.New("registration terminal CLOSE could not be emitted")
	failure := errors.Join(route.ErrClosedSourceCleanup, original)
	registered, err := owner.finishTextRegistration(context.Background(), flight, nil, nil, failure)
	if registered != nil || !errors.Is(err, original) {
		t.Fatalf("failed setup handed over or lost cause: %v", err)
	}
	if endpoint.textAvailable() {
		t.Fatal("cleanup failure left Endpoint accepting jobs")
	}
	select {
	case <-flight.done:
	default:
		t.Fatal("completion did not release flight")
	}
	if err := owner.Close(); !errors.Is(err, original) {
		t.Fatalf("context lost cleanup cause: %v", err)
	}
	if err := owner.Close(); !errors.Is(err, route.ErrClosedSourceCleanup) {
		t.Fatalf("repeated Close lost cleanup class: %v", err)
	}
	if err := endpoint.Close(); !errors.Is(err, original) {
		t.Fatalf("Endpoint lost cleanup cause: %v", err)
	}
}

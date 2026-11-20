package node

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestEmergencyPressureReportsExitAfterJoinedRole(t *testing.T) {
	var order []string
	record := func(value string) { order = append(order, value) }
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		record(event.State)
		return nil
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	config.cleanup = &dutyCleanup{retained: true, release: func() error {
		record("release")
		return nil
	}}
	joined := make(chan struct{})
	server := &dutyHandle{
		Joined: joined,
		Stop:   func() { record("stop") },
		Drain: func(context.Context) error {
			record("join")
			close(joined)
			return nil
		},
	}
	machine := stateMachine{current: stateReady}
	result, err := withdrawForResourcePressure(config, &machine, server, state.NodeDuty{Assignment: "closed_issuer"})
	want := []string{"DRAIN", "stop", "DRAINING", "join", "release", "WITHDRAWN", "EXIT"}
	if err != nil || result.State != "WITHDRAWN" || !slices.Equal(order, want) {
		t.Fatalf("resource drain result = %+v, %v with order %v, want %v", result, err, order, want)
	}
}

func TestEmergencyPressureDoesNotReportCompletedExitWhenDrainFails(t *testing.T) {
	drainErr := errors.New("role drain failed")
	var events []Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	machine := stateMachine{current: stateReady}
	server := &dutyHandle{Stop: func() {}, Drain: func(context.Context) error { return drainErr }}
	result, err := withdrawForResourcePressure(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"})
	if result.State != "FAILED" || !errors.Is(err, drainErr) {
		t.Fatalf("resource withdrawal = %+v, %v", result, err)
	}
	var resourceStates []string
	for _, event := range events {
		if event.Kind == "resource" {
			resourceStates = append(resourceStates, event.State)
		}
	}
	if len(resourceStates) != 1 || resourceStates[0] != "DRAIN" {
		t.Fatalf("resource states after failed drain = %v, want DRAIN without EXIT", resourceStates)
	}
}

func TestResourcePressureDeadlineHasSafeLifecycleReason(t *testing.T) {
	err := errors.Join(errors.New("private sampling failure"), context.DeadlineExceeded)
	if reason := resourcePressureFailureReason(err); reason != "resource pressure sampling timed out" {
		t.Fatalf("sampling deadline reason = %q", reason)
	}
}

//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
)

func TestTextParticipantObservationRetainsBackgroundDeliveryFailure(t *testing.T) {
	failure := errors.New("event output unavailable")
	output := newTextParticipantObservation(func(ctx context.Context, event TextParticipantEvent) error {
		if _, ok := ctx.Deadline(); !ok || event.Kind != "connection-operation-failed" || event.Failure != "service-result" {
			t.Error("background event lost its bounded context or safe category")
		}
		return failure
	})
	output.background(TextParticipantEvent{Kind: "connection-operation-failed", Failure: "service-result"})
	if err := output.pendingFailure(); !errors.Is(err, failure) {
		t.Fatalf("background delivery failure = %v", err)
	}
	if err := output.pendingFailure(); err != nil {
		t.Fatalf("failure returned twice: %v", err)
	}
}

func TestTextParticipantObservationForegroundFailureReturnsToCaller(t *testing.T) {
	failure := errors.New("event output unavailable")
	output := newTextParticipantObservation(func(context.Context, TextParticipantEvent) error { return failure })
	if err := output.emit(t.Context(), TextParticipantEvent{Kind: "ready"}); !errors.Is(err, failure) {
		t.Fatalf("foreground delivery failure = %v", err)
	}
	if err := output.pendingFailure(); err != nil {
		t.Fatalf("foreground failure was duplicated: %v", err)
	}
}

func TestTextParticipantObservationFailureEndsRuntimeWait(t *testing.T) {
	output := newTextParticipantObservation(func(context.Context, TextParticipantEvent) error { return errors.New("event output unavailable") })
	output.background(TextParticipantEvent{Kind: "publication-refresh-failed"})
	if err := output.wait(t.Context()); err == nil {
		t.Fatal("runtime remained ready after background event output failed")
	}
}

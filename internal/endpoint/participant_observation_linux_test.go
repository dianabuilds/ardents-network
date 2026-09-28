//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTextParticipantObservationRetainsBackgroundDeliveryFailure(t *testing.T) {
	failure := errors.New("event output unavailable")
	output := newParticipantObservation(func(ctx context.Context, event ClosedParticipantEvent) error {
		if _, ok := ctx.Deadline(); !ok || event.Kind != "connection-operation-failed" || event.Failure != "service-result" {
			t.Error("background event lost its bounded context or safe category")
		}
		return failure
	}, time.Now)
	output.background(ClosedParticipantEvent{Kind: "connection-operation-failed", Failure: "service-result"})
	if err := output.pendingFailure(); !errors.Is(err, failure) {
		t.Fatalf("background delivery failure = %v", err)
	}
	if err := output.pendingFailure(); err != nil {
		t.Fatalf("failure returned twice: %v", err)
	}
}

func TestTextParticipantObservationForegroundFailureReturnsToCaller(t *testing.T) {
	failure := errors.New("event output unavailable")
	output := newParticipantObservation(func(context.Context, ClosedParticipantEvent) error { return failure }, time.Now)
	if err := output.emit(t.Context(), ClosedParticipantEvent{Kind: "ready"}); !errors.Is(err, failure) {
		t.Fatalf("foreground delivery failure = %v", err)
	}
	if err := output.pendingFailure(); err != nil {
		t.Fatalf("foreground failure was duplicated: %v", err)
	}
}

func TestTextParticipantObservationFailureEndsRuntimeWait(t *testing.T) {
	output := newParticipantObservation(func(context.Context, ClosedParticipantEvent) error { return errors.New("event output unavailable") }, time.Now)
	output.background(ClosedParticipantEvent{Kind: "publication-refresh-failed"})
	if err := output.wait(t.Context()); err == nil {
		t.Fatal("runtime remained ready after background event output failed")
	}
}

func TestTextParticipantObservationRecordsOccurrenceBeforeOutput(t *testing.T) {
	at := time.Date(2026, time.September, 25, 12, 30, 0, 0, time.FixedZone("east", 3*60*60))
	var seen ClosedParticipantEvent
	output := newParticipantObservation(func(_ context.Context, event ClosedParticipantEvent) error {
		seen = event
		return nil
	}, func() time.Time { return at })
	if err := output.emit(t.Context(), ClosedParticipantEvent{Kind: "ready"}); err != nil {
		t.Fatal(err)
	}
	if !seen.At.Equal(at) || seen.At.Location() != time.UTC {
		t.Fatalf("event occurrence = %v", seen.At)
	}
}

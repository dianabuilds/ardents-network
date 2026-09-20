//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAwaitTextEndpointServiceObservationRetriesTransientFailure(t *testing.T) {
	attempts := 0
	err := awaitTextEndpointServiceObservation(t.Context(), func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient manager observation")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("got %d observations, want 3", attempts)
	}
}

func TestAwaitTextEndpointServiceObservationRemainsFailClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Millisecond)
	defer cancel()
	attempts := 0
	err := awaitTextEndpointServiceObservation(ctx, func(context.Context) error {
		attempts++
		return errors.New("permanent manager observation failure")
	})
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want bounded fail-closed result", err)
	}
	if attempts < 2 {
		t.Fatalf("got %d observations, want a retry", attempts)
	}
}

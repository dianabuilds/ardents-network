package nodeprincipal

import (
	"context"
	"testing"
)

func TestObserveRejectsInvalidOrCanceledCaller(t *testing.T) {
	if err := Observe(nil, 999, 999); err != ErrInput {
		t.Fatalf("nil caller: %v", err)
	}
	if err := Observe(context.Background(), 999, 998); err != ErrInput {
		t.Fatalf("different identities: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Observe(ctx, 999, 999); err != context.Canceled {
		t.Fatalf("caller cancellation: %v", err)
	}
}

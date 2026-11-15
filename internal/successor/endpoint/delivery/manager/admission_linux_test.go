package manager

import (
	"context"
	"errors"
	"testing"
)

func TestManagerRefusesUnknownUnitsAndCancelledCallerBeforeEffects(t *testing.T) {
	if _, err := State(context.Background(), "foreign.service"); !errors.Is(err, ErrInput) {
		t.Fatalf("foreign unit admitted: %v", err)
	}
	if _, _, err := Activation(context.Background(), "foreign"); !errors.Is(err, ErrInput) {
		t.Fatalf("foreign socket admitted: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, operation := range []func(context.Context) error{Start, Stop, Reload} {
		if err := operation(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled manager operation admitted: %v", err)
		}
	}
}

package platform

import (
	"context"
	"errors"
	"testing"
)

func TestPlatformObservationRetainsCancellationAndPhysicalErrors(t *testing.T) {
	if err := ObserveUbuntu2404(nil); !errors.Is(err, ErrBinding) {
		t.Fatalf("nil caller: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ObserveUbuntu2404(ctx); err != context.Canceled {
		t.Fatalf("caller cancellation lost: %v", err)
	}
	binding := errors.New("role binding")
	physical := errors.New("physical close")
	err := MapBinding(errors.Join(ErrBinding, physical), binding)
	if !errors.Is(err, binding) || !errors.Is(err, physical) || errors.Is(err, ErrBinding) {
		t.Fatalf("causes changed: %v", err)
	}
}

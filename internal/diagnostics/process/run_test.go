package process

import (
	"context"
	"errors"
	"testing"
)

func TestDisabledDiagnosticsPreserveWorkFailure(t *testing.T) {
	sentinel := errors.New("work failed")
	called := false
	err := Run(context.Background(), "", func(context.Context) error { called = true; return sentinel })
	if !called || !errors.Is(err, sentinel) {
		t.Fatalf("disabled outcome: %v, called=%v", err, called)
	}
}
func TestCanceledCallerDoesNotStartWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := Run(ctx, "", func(context.Context) error { called = true; return nil })
	if called || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled outcome: %v", err)
	}
}

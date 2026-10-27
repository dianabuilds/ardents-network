package systemd

import (
	"context"
	"testing"
)

func TestSourceReferenceClosePreservesOriginalCancellation(t *testing.T) {
	for _, test := range []struct {
		name     string
		canceled bool
		terminal error
		want     error
	}{
		{"live close", false, nil, ErrUnavailable},
		{"original cancellation", true, nil, context.Canceled},
		{"prior physical failure", true, ErrObservation, ErrObservation},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reference := &managerReference{original: ctx, terminal: test.terminal}
			if test.canceled {
				cancel()
			}
			reference.closed.Store(true)
			// Failure-only local admission controls grant no native observation.
			if err := reference.local(); err != test.want {
				t.Fatalf("local check replaced original result: got %v, want %v", err, test.want)
			}
			if err := reference.unavailable(); err != test.want {
				t.Fatalf("disconnection replaced original result: got %v, want %v", err, test.want)
			}
			if err := reference.begin(); err != test.want {
				t.Fatalf("new observation replaced original result: got %v, want %v", err, test.want)
			}
		})
	}
}

func TestSourceCanceledDeclarationPreservesOriginalCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	reference := &managerReference{original: ctx}
	cancel()
	// Failure-only admission: the canceled request must return before touching
	// a connection. An absent reply is not a malformed received declaration.
	_, _, err := reference.managerDeclaration(ctx)
	if err != context.Canceled {
		t.Fatalf("original canceled request invented a reply failure: %v", err)
	}
}

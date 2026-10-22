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
			reference := &sourcePreflight{original: ctx, terminal: test.terminal}
			if test.canceled {
				cancel()
			}
			reference.closed.Store(true)
			// Failure-only local admission controls grant no native observation.
			if err := reference.local(); err != test.want {
				t.Fatalf("local check replaced original result: got %v, want %v", err, test.want)
			}
			if err := reference.sourceUnavailable(); err != test.want {
				t.Fatalf("disconnection replaced original result: got %v, want %v", err, test.want)
			}
			if err := reference.begin(); err != test.want {
				t.Fatalf("new observation replaced original result: got %v, want %v", err, test.want)
			}
		})
	}
}

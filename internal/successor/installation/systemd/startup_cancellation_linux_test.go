package systemd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestStartupCallCancellationDoesNotHideForeignFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, err := range []error{net.ErrClosed, fmt.Errorf("original read: %w", net.ErrClosed)} {
		observed := startupCallFailure(ctx, err, true)
		if !errors.Is(observed, context.Canceled) || errors.Is(observed, net.ErrClosed) {
			t.Fatal("owned interruption invented an additional closed-socket failure", observed)
		}
	}
	for _, err := range []error{io.EOF, ErrObservation, errors.Join(net.ErrClosed, ErrObservation)} {
		if observed := startupCallFailure(ctx, err, true); observed != err {
			t.Fatal("interruption hid a genuine original failure", observed)
		}
	}
	if startupCallFailure(ctx, net.ErrClosed, false) != net.ErrClosed || startupCallFailure(t.Context(), net.ErrClosed, true) != net.ErrClosed {
		t.Fatal("foreign or uncanceled close became owned cancellation")
	}
	expired, stop := context.WithDeadline(t.Context(), time.Unix(1, 0))
	defer stop()
	if !errors.Is(startupCallFailure(expired, net.ErrClosed, true), context.DeadlineExceeded) {
		t.Fatal("original deadline became ordinary cancellation")
	}
}

func TestStartupCanceledManagerReadIsNotChangedIdentity(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// A canceled original read cannot supply any successful manager fact. No
	// bus or accepting observation is injected; request refuses before I/O.
	client := &startupBus{}
	if err := client.sameOwner(ctx, ":1.32"); !errors.Is(err, context.Canceled) || errors.Is(err, ErrObservation) {
		t.Fatal("unobserved manager became a changed identity", err)
	}
}

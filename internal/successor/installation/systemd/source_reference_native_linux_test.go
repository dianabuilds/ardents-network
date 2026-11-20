//go:build network_signing_native

package systemd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// Failure-only comparisons of the actual pinned library sentinel. No manager,
// connection, successful observation or Source/Forwarding authority is supplied.
func TestSourceReferenceLibraryCloseCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, err := range []error{dbus.ErrClosed, fmt.Errorf("original call: %w", dbus.ErrClosed)} {
		if observed := startupCallFailure(ctx, err, true); observed != context.Canceled {
			t.Fatal("original D-Bus interruption invented a second failure", observed)
		}
	}
	for _, err := range []error{io.EOF, ErrObservation, errors.Join(dbus.ErrClosed, ErrObservation), errors.New(dbus.ErrClosed.Error())} {
		if observed := startupCallFailure(ctx, err, true); observed != err {
			t.Fatal("original interruption hid an independent failure", observed)
		}
	}
	if startupCallFailure(ctx, dbus.ErrClosed, false) != dbus.ErrClosed || startupCallFailure(t.Context(), dbus.ErrClosed, true) != dbus.ErrClosed {
		t.Fatal("foreign or uncanceled D-Bus close became owned cancellation")
	}
	expired, stop := context.WithDeadline(t.Context(), time.Unix(1, 0))
	defer stop()
	if startupCallFailure(expired, dbus.ErrClosed, true) != context.DeadlineExceeded {
		t.Fatal("original D-Bus deadline became ordinary cancellation")
	}
}

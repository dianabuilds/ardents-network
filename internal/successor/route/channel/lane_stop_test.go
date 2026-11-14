package channel

import (
	"context"
	"errors"
	"testing"
)

// A borrowed control lane (AcceptParent) carries the caller context without
// owning a cancel. Sealing it must refuse and retain the cause, not panic
// (audit RD-7).
func TestStopLockedWithoutCancelRefusesWithoutPanic(t *testing.T) {
	l := &Lane{changed: make(chan struct{})}
	cause := errors.New("route charge refused")
	l.stopLocked(cause)
	if !l.closed || !errors.Is(l.cause, cause) {
		t.Fatal("cancel-less lane did not retain the refusal")
	}
	l.stopLocked(nil)
	if !errors.Is(l.cause, cause) {
		t.Fatal("repeated stop changed the retained cause")
	}
}

// Lanes that own a cancel keep cancelling it with the refusal cause.
func TestStopLockedWithCancelStillCancels(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	l := &Lane{changed: make(chan struct{}), ctx: ctx, cancel: cancel}
	cause := errors.New("route charge refused")
	l.stopLocked(cause)
	if !errors.Is(context.Cause(ctx), cause) {
		t.Fatal("owned cancel was not invoked with the refusal cause")
	}
}

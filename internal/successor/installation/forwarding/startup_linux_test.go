package forwarding

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestForwardingStartupRootAndEmptyHandlesConveyNoRuntimeRight(t *testing.T) {
	if os.Geteuid() == 0 {
		h, err := OpenStartup(t.Context(), "/var/lib/ardents-node-installation/binding.json")
		if h != nil || !errors.Is(err, ErrInput) {
			_ = h.Close()
			t.Fatal("root observation supplied independent non-root Startup", err)
		}
	}
	var h *Startup
	if !errors.Is(h.Check(), ErrInput) || h.Context() != nil || h.Close() != nil {
		t.Fatal("nil Startup conveyed an observation")
	}
	if inputs, err := h.Inputs(); !errors.Is(err, ErrInput) || len(inputs.Plan) != 0 {
		t.Fatal("nil Startup released private data", err)
	}
	h.Stop()
}

func TestForwardingStartupLocalDenialCannotReviveOriginalCaller(t *testing.T) {
	// Local latch test only. No successful manager/process/files/ACK, accepting
	// Startup or domain authority is fabricated by this private state fixture.
	ctx, cancel := context.WithCancel(t.Context())
	s := &startup{ctx: ctx, parent: t.Context(), cancel: cancel, ready: true}
	h := &Startup{original: s}
	copy := *h
	h.Stop()
	if !errors.Is(copy.original.begin(), context.Canceled) || !errors.Is(copy.original.local(), context.Canceled) {
		t.Fatal("copied Startup revived after synchronous Stop")
	}
	if err := copy.original.record(os.ErrClosed); !errors.Is(err, os.ErrClosed) {
		t.Fatal("actual late physical cause lost", err)
	}
	if err := h.original.begin(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("late failure was replaced by caller cancellation", err)
	}
}

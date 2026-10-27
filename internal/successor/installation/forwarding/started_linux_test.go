package forwarding

import (
	"context"
	"errors"
	"testing"
)

func TestForwardingStartedEmptyAndCanceledHandlesNeverSupplyHistory(t *testing.T) {
	var empty Started
	if !errors.Is(empty.Check(), ErrInput) || empty.Close() != nil {
		t.Fatal("empty start observation supplied history or cleanup")
	}
	if h, err := ReadStarted(t.Context(), nil, "/opt/unopened/binding.json", "/var/lib/unopened-history"); !errors.Is(err, ErrInput) || h.original != nil {
		t.Fatal("missing verifier supplied start history", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Local denial only: no native snapshot, verifier, completed installation or
	// accepting historical/runtime authority is fabricated by this canceled owner.
	h := Started{original: &started{ctx: ctx}}
	copy := h
	if !errors.Is(h.Check(), context.Canceled) {
		t.Fatal("original caller denial lost")
	}
	if !errors.Is(copy.Close(), context.Canceled) || !errors.Is(h.Close(), context.Canceled) {
		t.Fatal("copied close erased original caller denial")
	}
}

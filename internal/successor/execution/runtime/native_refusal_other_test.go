//go:build !linux

package runtime

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

func TestUnsupportedNativeLaunchJoinsRefusedLocalJobWithoutPoisoningGeneration(t *testing.T) {
	principal := [32]byte{2}
	owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: execution.Connection}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
	})
	// More than the finite 64-session capacity distinguishes original joined
	// refusal from leaked reservations or a poisoned generation.
	for attempt := 0; attempt < 70; attempt++ {
		prepared, err := owner.Prepare(t.Context(), principal, execution.Connection)
		if prepared != nil || err == nil || !strings.Contains(err.Error(), "native platform is unavailable") {
			t.Fatal("unsupported native worker supplied preparation provenance")
		}
		invocation, err := owner.Launch(t.Context(), principal, execution.Connection)
		if invocation != nil || err == nil || !strings.Contains(err.Error(), "native platform is unavailable") {
			t.Fatal("unsupported native worker supplied a qualified invocation")
		}
		if len(launchGate) != 0 {
			t.Fatal("refused original launch retained fixed activation serialization")
		}
	}
}

//go:build linux || darwin

package networkfixture

import (
	"crypto/ed25519"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// OpenIndependentState accepts the same signed inputs under a separate root,
// lease and clock observation. It shares no mutable State with the caller.
func OpenIndependentState(t testing.TB, spec networkfixture.ClosedSpec, expectedEpoch [32]byte) *state.State {
	t.Helper()
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Epoch.Digest != expectedEpoch {
		t.Fatal("independently encoded Epoch differs from the original")
	}
	clock, stop := StartObservedClock(t)
	t.Cleanup(stop)
	var key [32]byte
	copy(key[:], spec.Authority.Public().(ed25519.PublicKey))
	plan := SeedNetworkState(t, spec.NetworkID, key, bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	return owner
}

package networkfixture

import (
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// SeedNetworkState publishes the signed test Epoch and profile through State.
// It closes the original owner before the command opens that history.
func SeedNetworkState(t testing.TB, networkID, key [32]byte, bundle networkfixture.Closed, clock string) state.AuthorityDeclaration {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal("resolve State fixture directory", err)
	}
	plan := state.AuthorityDeclaration{Root: filepath.Join(root, "network"), NetworkID: networkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: clock}
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptEpoch(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	return plan
}

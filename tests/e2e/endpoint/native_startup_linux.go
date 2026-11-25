//go:build linux && document_worker_installed

package endpoint

import (
	"crypto/ed25519"
	"testing"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The root driver runs outside the Endpoint unit. Its actual pinned program is
// the non-root Endpoint MainPID; only genuine Installation commands create the
// account, fixed resources, Root completion exchange and selected generation.
// All signing, Source and clock fixtures have one controller. This proves no
// independent custody, trusted time, private Service or whole-host qualification.

// BuildSignedNetwork encodes the fixture’s public Network inputs and trust pins.
func BuildSignedNetwork(t *testing.T, spec networkfixture.ClosedSpec) SignedNetwork {
	t.Helper()
	epoch, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], spec.Authority.Public().(ed25519.PublicKey))
	return SignedNetwork{NetworkID: spec.NetworkID, Seed: spec.Seed, Authority: authority,
		Epoch: epoch.Epoch.Raw, Inputs: epoch.Epoch.Inputs, Materials: epoch.Epoch.Materials, Profile: epoch.Profile}
}

// SignedNetwork contains only public signed material and trust pins.
// Every participant still authenticates it through its own actual State;
// no live State, signing key, qualified operation or Instance crosses managers.
type SignedNetwork struct {
	NetworkID, Seed, Authority [32]byte
	Epoch, Profile             []byte
	Inputs, Materials          [][]byte
}

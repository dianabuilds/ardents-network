package networkfixture

import (
	"crypto/ed25519"
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	epochfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestSignedGenesisAdmitsAndReopensOriginalState(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	spec, issuer, inventory := PrepareNetworkGenesis(t, root)
	bundle, err := epochfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal("build independently signed Network", err)
	}
	PrepareIssuer(t, &issuer, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
	clock, stopClock := StartObservedClock(t)
	defer stopClock()
	var authority [32]byte
	copy(authority[:], spec.Authority.Public().(ed25519.PublicKey))
	plan := SeedNetworkState(t, spec.NetworkID, authority, bundle, clock)
	config, err := plan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal("reopen original State", err)
	}
	view, viewErr := owner.CurrentRuntime()
	closeErr := owner.Close()
	if viewErr != nil || closeErr != nil {
		t.Fatal("observe reopened State", viewErr, closeErr)
	}
	member, err := view.Member(spec.Nodes[1].NodeID, view.ObservedAt())
	if err != nil || member.DutyGeneration != spec.Nodes[1].Generation {
		t.Fatal("signed receiver duty changed on reopen", err)
	}
}

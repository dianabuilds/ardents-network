package networkfixture_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	stateepoch "github.com/dianabuilds/ardents-network/internal/network/epoch"
	epochfixture "github.com/dianabuilds/ardents-network/tests/fixtures/networkfixture"
)

func TestBuildEpochProducesCanonicalVerifiableEvidence(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	network := sha256.Sum256([]byte("network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	record, err := epochfixture.BuildRecord(epochfixture.RecordSpec{NetworkID: network, NodeID: sha256.Sum256([]byte("node")), Generation: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Family: "family", Endpoint: "127.0.0.1:1", Capability: 1, Capacity: 1, PrivateKey: node})
	if err != nil {
		t.Fatal(err)
	}
	epoch, err := epochfixture.BuildEpoch(epochfixture.EpochSpec{NetworkID: network, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Inputs: [][]byte{record.Raw}, Accepted: []epochfixture.Record{record}, AssignmentSeed: [32]byte{3}, Domains: []string{"alpha"}, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	id := sha256.Sum256(authority.Public().(ed25519.PublicKey))
	if _, err := stateepoch.Verify(stateepoch.Policy{NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{id: authority.Public().(ed25519.PublicKey)}, Threshold: 1, Now: now}, epoch.Raw, epoch.Inputs, epoch.Materials, true); err != nil {
		t.Fatal(err)
	}
}

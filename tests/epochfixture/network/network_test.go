package network_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	fixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestCanonicalNetworkFixtureIsStableAndRejectsIncompleteInput(t *testing.T) {
	networkID := sha256.Sum256([]byte("qualification-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	nodeID := sha256.Sum256(nodeKey.Public().(ed25519.PublicKey))
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	spec := fixture.RecordSpec{NetworkID: networkID, NodeID: nodeID, Generation: 1,
		ValidFrom: now, ValidUntil: now.Add(24 * time.Hour), Family: "qualification-node-01",
		Endpoint: "192.0.2.10:48000", Carrier: "ardents-carrier-tcp-tls-v2",
		Capability: 2, Capacity: 4, PrivateKey: nodeKey}
	first, err := fixture.BuildRecord(spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.BuildRecord(spec)
	if err != nil || !bytes.Equal(first.Raw, second.Raw) {
		t.Fatalf("canonical record changed across an exact rebuild: %v", err)
	}
	epochSpec := fixture.EpochSpec{NetworkID: networkID, Number: 1, ValidFrom: now,
		ValidUntil: now.Add(24 * time.Hour), Inputs: [][]byte{first.Raw}, Accepted: []fixture.Record{first},
		AssignmentSeed: sha256.Sum256([]byte("qualification-assignment")), Profile: "ardents-route-v3",
		Version: 3, Domains: []string{"qualification"}, Authorities: []ed25519.PrivateKey{authority}}
	epoch, err := fixture.BuildEpoch(epochSpec)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := fixture.BuildEpoch(epochSpec)
	if err != nil || !bytes.Equal(epoch.Raw, rebuilt.Raw) || epoch.Digest != rebuilt.Digest ||
		len(epoch.Inputs) != 1 || len(epoch.Materials) != 1 {
		t.Fatalf("canonical Epoch changed across an exact rebuild: %v", err)
	}
	if _, err := fixture.BuildRecord(fixture.RecordSpec{}); err == nil {
		t.Fatal("incomplete Node Record fixture was accepted")
	}
	if _, err := fixture.BuildEpoch(fixture.EpochSpec{}); err == nil {
		t.Fatal("incomplete Epoch fixture was accepted")
	}
}

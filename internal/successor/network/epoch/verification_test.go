package epoch_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestVerifyReturnsAuthenticatedHeaderAndCandidate(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	networkID := sha256.Sum256([]byte("epoch verifier package test"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, ed25519.SeedSize))
	record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
		NetworkID: networkID, NodeID: [32]byte{1}, Generation: 1,
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(2 * time.Hour),
		Family: "alpha-family", Endpoint: "127.0.0.1:4321",
		Carrier: epoch.CarrierLegacyTCP, Capability: 1, Capacity: 1,
		PrivateKey: nodeKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	built, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{
		NetworkID: networkID, Number: 1, ValidFrom: now.Add(-time.Minute),
		ValidUntil: now.Add(time.Hour), Inputs: [][]byte{record.Raw},
		Accepted: []networkfixture.Record{record}, Domains: []string{"alpha"},
		Authorities: []ed25519.PrivateKey{authority}, Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	policy := epoch.Policy{
		NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{
			sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey),
		},
		Threshold: 1, Profile: epoch.ProfileRoleProbe, Now: now,
	}
	decision, err := epoch.Verify(policy, built.Raw, built.Inputs, built.Materials[:1], true)
	if err != nil {
		t.Fatalf("verify signed fixture: %v", err)
	}
	if decision.Header.Number != 1 || decision.Header.Digest != built.Digest ||
		decision.Header.Version != 1 || decision.Snapshot.Digest != built.Digest ||
		len(decision.Candidates) != 1 || decision.Candidates[0].NodeID != record.NodeID ||
		decision.Candidates[0].RecordDigest != sha256.Sum256(record.Raw) {
		t.Fatalf("verified decision lost authenticated identity: %+v", decision.Header)
	}
	material, err := decision.Materialization(0)
	if err != nil || !bytes.Equal(material, built.Materials[0]) {
		t.Fatalf("materialization differs from independent fixture: %v", err)
	}
	tampered := append([]byte(nil), built.Raw...)
	tampered[len(tampered)-1] ^= 1
	if header, err := epoch.Inspect(tampered); err != nil || header.Digest != built.Digest {
		t.Fatalf("bounded untrusted inspection: %+v / %v", header, err)
	}
	if _, err := epoch.Verify(policy, tampered, built.Inputs, built.Materials[:1], true); err == nil {
		t.Fatal("accepted an inspected header with a forged signature")
	}
}

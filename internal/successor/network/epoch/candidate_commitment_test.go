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

func TestClosedCandidateDispositionMatchesIndependentSignedCommitment(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network := sha256.Sum256([]byte("candidate disposition commitment"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	base := networkfixture.RecordSpec{NetworkID: network, NodeID: [32]byte{1}, Generation: 1,
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), Family: "family", Endpoint: "127.0.0.1:4321",
		Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 17, PrivateKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, ed25519.SeedSize))}
	valid, err := networkfixture.BuildRecord(base)
	if err != nil {
		t.Fatal(err)
	}
	badSignature := append([]byte(nil), valid.Raw...)
	badSignature[len(badSignature)-1] ^= 1
	wrongNetwork := base
	wrongNetwork.NetworkID = [32]byte{9}
	wrong, err := networkfixture.BuildRecord(wrongNetwork)
	if err != nil {
		t.Fatal(err)
	}
	wrong.Raw[len(wrong.Raw)-1] ^= 1 // Network refusal must precede bad signature.
	expired := base
	expired.ValidUntil = now
	expired.Capability = 1
	timeInvalid, err := networkfixture.BuildRecord(expired)
	if err != nil {
		t.Fatal(err)
	}
	carrier := base
	carrier.Carrier = epoch.CarrierQUIC
	carrier.PrivateKey = authority
	carrierInvalid, err := networkfixture.BuildRecord(carrier)
	if err != nil {
		t.Fatal(err)
	}
	inputs := [][]byte{valid.Raw, []byte("ARNR"), wrong.Raw, badSignature, timeInvalid.Raw, carrierInvalid.Raw}
	// No policy implementation is used to build this signed expected View.
	reasons := map[uint32]uint16{1: 1, 2: 2, 3: 3, 4: 4, 5: 12}
	spec := networkfixture.EpochSpec{NetworkID: network, Number: 1, ValidFrom: now, ValidUntil: now.Add(30 * time.Minute),
		Profile: epoch.ProfileClosedRoute, Version: 3, Inputs: inputs, Accepted: []networkfixture.Record{valid}, Rejections: reasons,
		AssignmentSeed: [32]byte{2}, Domains: []string{"initiator", "introduction", "rendezvous", "responder"}, Authorities: []ed25519.PrivateKey{authority}}
	policy := epoch.Policy{NetworkID: network, Now: now, Profile: epoch.ProfileClosedRoute, Threshold: 1,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey)}}
	built, err := networkfixture.BuildEpoch(spec)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := epoch.Verify(policy, built.Raw, inputs, built.Materials, true)
	if err != nil || len(decision.Candidates) != 1 || decision.Candidates[0].NodeID != valid.NodeID {
		t.Fatalf("independent signed disposition refused: %v", err)
	}
	reasons[5] = 7 // Signed but wrong Carrier/authority priority must refuse.
	wrongCommitment, err := networkfixture.BuildEpoch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := epoch.Verify(policy, wrongCommitment.Raw, inputs, wrongCommitment.Materials, true); err == nil {
		t.Fatal("incorrect signed rejection precedence admitted")
	}
	duplicate := base
	duplicate.Generation = 2
	duplicate.Endpoint = "127.0.0.1:4322"
	duplicateRecord, err := networkfixture.BuildRecord(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	spec.Inputs, spec.Accepted, spec.Rejections = [][]byte{valid.Raw, duplicateRecord.Raw}, nil, map[uint32]uint16{0: 8, 1: 8}
	collision, err := networkfixture.BuildEpoch(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := epoch.Verify(policy, collision.Raw, collision.Inputs, nil, false); err != nil {
		t.Fatalf("all colliding Nodes must be excluded: %v", err)
	}
}

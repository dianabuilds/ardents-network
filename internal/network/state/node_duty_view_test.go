package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestCurrentNodeDutyProjectsSignedCarrierProfilesAndRejectsUnknown(t *testing.T) {
	now := time.Unix(fixtureNow, 0).UTC()
	network := sha256.Sum256([]byte("carrier-profile-state"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xc1}, ed25519.SeedSize))
	firstKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xc2}, ed25519.SeedSize))
	secondKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xc3}, ed25519.SeedSize))
	rejectedKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xc4}, ed25519.SeedSize))
	first := buildTestRecordWithCapability(t, network, [32]byte{1}, firstKey, "carrier-family-a", "127.0.0.1:45101",
		2, 2, now.Add(-time.Minute), now.Add(time.Hour))
	second := buildTestRecordWithCarrier(t, network, [32]byte{2}, secondKey, "carrier-family-b", "127.0.0.1:45102",
		"ardents-carrier-quic-v1", 2, 2, now.Add(-time.Minute), now.Add(time.Hour))
	unknown := buildTestRecordWithCarrier(t, network, [32]byte{3}, rejectedKey, "carrier-family-c", "127.0.0.1:45103",
		"ardents-carrier-unknown-v1", 2, 2, now.Add(-time.Minute), now.Add(time.Hour))
	spec := testEpochSpec{networkID: network, number: 1, validFrom: now.Add(-30 * time.Second), validUntil: now.Add(30 * time.Minute),
		inputs: [][]byte{first.bytes, second.bytes, unknown.bytes}, accepted: []fixtureRecord{first, second}, rejections: map[uint32]uint16{2: 12},
		assignmentSeed: sha256.Sum256([]byte("carrier-assignment")), domains: []string{"initiator", "rendezvous"},
		authorities: []ed25519.PrivateKey{authority}, profile: "ardents-interactive-route-v1", version: 1}
	epoch := buildTestEpoch(t, spec)
	opened, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey)},
		Threshold:   1, Now: now, AcceptedProfile: "ardents-interactive-route-v1"})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Accept(context.Background(), epoch.Raw, spec.inputs, epoch.Materials[:1]); err != nil {
		t.Fatal(err)
	}
	view, err := opened.CurrentNodeDuty()
	if err != nil {
		t.Fatal(err)
	}
	if view.DutyCarrierProfile() != "ardents-carrier-tcp-tls-v1" || view.DutyCandidateCount() != 2 ||
		view.DutyCandidateCarrierProfile(0) != "ardents-carrier-tcp-tls-v1" ||
		view.DutyCandidateCarrierProfile(1) != "ardents-carrier-quic-v1" || view.DutyCandidateCarrierProfile(2) != "" {
		t.Fatalf("signed Carrier projection is incomplete: own=%q first=%q second=%q count=%d", view.DutyCarrierProfile(),
			view.DutyCandidateCarrierProfile(0), view.DutyCandidateCarrierProfile(1), view.DutyCandidateCount())
	}
}

func TestCurrentNodeDutyExposesOnlyCurrentAuthenticatedDutyFacts(t *testing.T) {
	value := newFixture(t)
	opened, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0)})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Accept(context.Background(), value.epoch, value.inputs, value.materializations); err != nil {
		t.Fatal(err)
	}
	view, err := opened.CurrentNodeDuty()
	if err != nil {
		t.Fatal(err)
	}
	if view.DutyNetworkID() != value.networkID || view.DutyEpoch() != 1 || view.DutyDigest() != value.epochDigest ||
		!view.DutyRecordPresent() || view.DutyAssignment() == "" || view.DutyProbeCapacity() == 0 {
		t.Fatalf("Node duty view lacks authenticated duty facts")
	}
	if view.DutyCandidateCount() != 2 || view.DutyCandidateNodeID(0) != value.accepted[0].nodeID ||
		view.DutyCandidatePublicKey(0) == [32]byte{} || view.DutyCandidateEndpoint(0) == "" ||
		view.DutyCandidateAssignment(0) == "" || view.DutyCandidateValidFrom(0).IsZero() ||
		view.DutyCandidateValidUntil(0).IsZero() {
		t.Fatalf("Node duty view lacks authenticated candidate facts")
	}
	if view.DutyCandidateKeyID(0) == [32]byte{} || view.DutyCandidateFamilyID(0) == [32]byte{} ||
		view.DutyCandidateRecordDigest(0) == [32]byte{} || view.DutyCandidateDomainProofDigest(0) == [32]byte{} ||
		view.DutyCandidateCapacity(0) == 0 || view.DutyCandidateAssignmentNotAfter(0).IsZero() {
		t.Fatal("Node duty view lacks the bounded Entry-verification candidate facts")
	}
	if view.DutyAuthorityCount() != 1 || view.DutyAuthorityID(0) != value.authorityID ||
		view.DutyAuthorityPublicKey(0) != [32]byte(value.authorityPublic) {
		t.Fatal("Node duty view lacks the current State authority verification fact")
	}
	if view.DutyAuthorityID(1) != [32]byte{} || view.DutyAuthorityPublicKey(1) != [32]byte{} {
		t.Fatal("Node duty view exposed an out-of-range State authority")
	}
	if view.DutyCandidateNodeID(2) != [32]byte{} || view.DutyCandidateEndpoint(2) != "" ||
		!view.DutyCandidateValidUntil(2).IsZero() {
		t.Fatal("Node duty view exposed an out-of-range candidate")
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.CurrentNodeDuty(); err == nil {
		t.Fatal("closed Network State exposed a Node duty view")
	}
}

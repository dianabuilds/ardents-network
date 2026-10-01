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
		authorities: []ed25519.PrivateKey{authority}, profile: "ardents-interactive-route-v2", version: 1}
	epoch := buildTestEpoch(t, spec)
	opened, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey)},
		Threshold:   1, Now: now, AcceptedProfile: "ardents-interactive-route-v2"})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Accept(context.Background(), epoch.Raw, spec.inputs, epoch.Materials[:1]); err != nil {
		t.Fatal(err)
	}
	duty, err := opened.CurrentNodeDuty()
	if err != nil {
		t.Fatal(err)
	}
	if duty.CarrierProfile != "ardents-carrier-tcp-tls-v1" || duty.CandidateCount != 2 ||
		duty.Candidates[0].CarrierProfile != "ardents-carrier-tcp-tls-v1" ||
		duty.Candidates[1].CarrierProfile != "ardents-carrier-quic-v1" || duty.Candidates[2].CarrierProfile != "" {
		t.Fatalf("signed Carrier projection is incomplete: own=%q first=%q second=%q count=%d", duty.CarrierProfile,
			duty.Candidates[0].CarrierProfile, duty.Candidates[1].CarrierProfile, duty.CandidateCount)
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
	duty, err := opened.CurrentNodeDuty()
	if err != nil {
		t.Fatal(err)
	}
	if duty.NetworkID != value.networkID || duty.Epoch != 1 || duty.Digest != value.epochDigest ||
		!duty.RecordPresent || duty.Assignment == "" || duty.ProbeCapacity == 0 {
		t.Fatalf("Node duty value lacks authenticated duty facts")
	}
	if duty.CandidateCount != 2 || duty.Candidates[0].NodeID != value.accepted[0].nodeID ||
		duty.Candidates[0].PublicKey == [32]byte{} || duty.Candidates[0].Endpoint == "" ||
		duty.Candidates[0].Assignment == "" || duty.Candidates[0].ValidFrom.IsZero() ||
		duty.Candidates[0].ValidUntil.IsZero() {
		t.Fatalf("Node duty value lacks authenticated candidate facts")
	}
	if duty.Candidates[0].KeyID == [32]byte{} || duty.Candidates[0].FamilyID == [32]byte{} ||
		duty.Candidates[0].RecordDigest == [32]byte{} || duty.Candidates[0].DomainProofDigest == [32]byte{} ||
		duty.Candidates[0].Capacity == 0 || duty.Candidates[0].AssignmentNotAfter.IsZero() {
		t.Fatal("Node duty value lacks the bounded Entry-verification candidate facts")
	}
	if duty.Candidates[2].NodeID != [32]byte{} || duty.Candidates[2].Endpoint != "" ||
		!duty.Candidates[2].ValidUntil.IsZero() {
		t.Fatal("Node duty value exposed an out-of-range candidate")
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.CurrentNodeDuty(); err == nil {
		t.Fatal("closed Network State exposed a Node duty value")
	}
}

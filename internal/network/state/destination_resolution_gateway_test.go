package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestResolutionViewProjectsOnlyItsSignedDestinationGateway(t *testing.T) {
	t.Parallel()
	now := time.Unix(fixtureNow, 0).UTC()
	network := sha256.Sum256([]byte("destination-resolution-state"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa8}, ed25519.SeedSize))
	domains := []string{"destination-resolution", "initiator", "introduction", "rendezvous", "responder"}
	seed := sha256.Sum256([]byte("destination-resolution-assignment"))
	records := routeProfileRecords(t, network, seed, domains, now)
	var selected fixtureRecord
	for _, record := range records {
		domain, err := fixtureAssignmentDomain(network, 1, seed, record.family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if domain == "destination-resolution" {
			selected = record
		}
	}
	if selected.nodeID == [32]byte{} {
		t.Fatal("fixture did not produce a Destination Resolution Gateway")
	}
	profile := []byte("one State-associated signed GatewayProfile")
	spec := testEpochSpec{networkID: network, number: 1, validFrom: now.Add(-time.Minute), validUntil: now.Add(time.Hour),
		inputs: recordBytes(records), accepted: records, rejections: map[uint32]uint16{}, assignmentSeed: seed, domains: domains,
		authorities: []ed25519.PrivateKey{authority}, profile: "ardents-interactive-route-v2", version: 2,
		destinationGateway: selected.nodeID, destinationProfile: profile}
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
	view, err := opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	got, available := view.Gateway(now, now.Add(5*time.Second))
	if !available || got.NodeID != selected.nodeID || got.Family != sha256.Sum256([]byte(selected.family)) ||
		string(got.Profile) != string(profile) {
		t.Fatalf("Gateway = %+v, available=%t", got, available)
	}
	got.Profile[0] ^= 1
	again, available := view.Gateway(now, now.Add(5*time.Second))
	if !available || string(again.Profile) != string(profile) {
		t.Fatal("caller mutation changed the State-owned Gateway profile")
	}
	if _, available := view.Gateway(now, now.Add(16*time.Second)); available {
		t.Fatal("Gateway accepted a resolution window outside State's fixed bound")
	}

	bad := spec
	for _, record := range records {
		if record.nodeID != selected.nodeID {
			bad.destinationGateway = record.nodeID
			break
		}
	}
	badEpoch := buildTestEpoch(t, bad)
	rejected, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey)},
		Threshold:   1, Now: now, AcceptedProfile: "ardents-interactive-route-v2"})
	if err != nil {
		t.Fatal(err)
	}
	defer rejected.Close()
	if _, err := rejected.Accept(context.Background(), badEpoch.Raw, bad.inputs, badEpoch.Materials[:1]); err == nil {
		t.Fatal("State accepted a Gateway profile associated with a different Node")
	}
}

func TestResolutionViewProjectsOnlyItsSignedTransitIssuer(t *testing.T) {
	t.Parallel()
	now := time.Unix(fixtureNow, 0).UTC()
	network := sha256.Sum256([]byte("transit-issuance-state"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	domains := []string{"destination-resolution", "initiator", "introduction", "rendezvous", "responder", "transit-issuance"}
	seed := sha256.Sum256([]byte("transit-issuance-assignment"))
	records := routeProfileRecords(t, network, seed, domains, now)
	var selected fixtureRecord
	for _, record := range records {
		domain, err := fixtureAssignmentDomain(network, 1, seed, record.family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if domain == "transit-issuance" {
			selected = record
		}
	}
	if selected.nodeID == [32]byte{} {
		t.Fatal("fixture did not produce a transit issuer")
	}
	spec := testEpochSpec{networkID: network, number: 1, validFrom: now.Add(-time.Minute), validUntil: now.Add(time.Hour),
		inputs: recordBytes(records), accepted: records, rejections: map[uint32]uint16{}, assignmentSeed: seed, domains: domains,
		authorities: []ed25519.PrivateKey{authority}, profile: "ardents-interactive-route-v2", version: 3,
		destinationGateway: records[0].nodeID, destinationProfile: []byte("selected gateway profile"),
		transitIssuer: selected.nodeID, transitIssuerProfile: []byte("one State-associated signed transit issuer profile")}
	for _, record := range records {
		domain, err := fixtureAssignmentDomain(network, 1, seed, record.family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if domain == "destination-resolution" {
			spec.destinationGateway = record.nodeID
		}
	}
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
	view, err := opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	issuer, available := view.CredentialIssuer(now, now.Add(5*time.Second))
	if !available || issuer.NodeID != selected.nodeID || issuer.Family != sha256.Sum256([]byte(selected.family)) ||
		string(issuer.Profile) != string(spec.transitIssuerProfile) {
		t.Fatalf("CredentialIssuer = %+v, available=%t", issuer, available)
	}
	issuer.Profile[0] ^= 1
	again, available := view.CredentialIssuer(now, now.Add(5*time.Second))
	if !available || string(again.Profile) != string(spec.transitIssuerProfile) {
		t.Fatal("caller mutation changed State-owned transit issuer profile")
	}
	bad := spec
	for _, record := range records {
		if record.nodeID != selected.nodeID {
			bad.transitIssuer = record.nodeID
			break
		}
	}
	badEpoch := buildTestEpoch(t, bad)
	rejected, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey)},
		Threshold:   1, Now: now, AcceptedProfile: "ardents-interactive-route-v2"})
	if err != nil {
		t.Fatal(err)
	}
	defer rejected.Close()
	if _, err := rejected.Accept(context.Background(), badEpoch.Raw, bad.inputs, badEpoch.Materials[:1]); err == nil {
		t.Fatal("State accepted a transit issuer profile associated with a different Node")
	}
}

func routeProfileRecords(t *testing.T, network, seed [32]byte, domains []string, now time.Time) []fixtureRecord {
	t.Helper()
	records := make([]fixtureRecord, 0, len(domains))
	selected := make(map[string]bool, len(domains))
	for marker := byte(1); len(records) < len(domains) && marker < 128; marker++ {
		family := fmt.Sprintf("route-family-%d", marker)
		domain, err := fixtureAssignmentDomain(network, 1, seed, family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if selected[domain] {
			continue
		}
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
		nodeID := sha256.Sum256([]byte{0xd0, marker})
		records = append(records, buildTestRecordWithCapability(t, network, nodeID, private, family,
			fmt.Sprintf("127.0.0.1:%d", 42000+int(marker)), 1, 2, now.Add(-time.Minute), now.Add(2*time.Hour)))
		selected[domain] = true
	}
	if len(records) != len(domains) {
		t.Fatal("could not construct one candidate for every native Route domain")
	}
	return records
}

func recordBytes(records []fixtureRecord) [][]byte {
	result := make([][]byte, len(records))
	for index, record := range records {
		result[index] = record.bytes
	}
	return result
}

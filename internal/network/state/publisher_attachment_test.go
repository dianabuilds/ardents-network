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

func TestResolutionViewProjectsOneCompletePublisherAttachment(t *testing.T) {
	t.Parallel()
	now := time.Unix(fixtureNow, 0).UTC()
	deadline := now.Add(5 * time.Second)
	network := sha256.Sum256([]byte("publisher-attachment-state"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xaa}, ed25519.SeedSize))
	domains := []string{"initiator", "introduction", "rendezvous", "responder"}
	seed := sha256.Sum256([]byte("publisher-attachment-assignment"))
	records := routeProfileRecords(t, network, seed, domains, now)
	expected := make(map[string]fixtureRecord, len(domains))
	for _, record := range records {
		domain, err := fixtureAssignmentDomain(network, 1, seed, record.family, domains)
		if err != nil {
			t.Fatal(err)
		}
		expected[domain] = record
	}
	spec := testEpochSpec{networkID: network, number: 1, validFrom: now.Add(-time.Minute), validUntil: now.Add(time.Hour),
		inputs: recordBytes(records), accepted: records, rejections: map[uint32]uint16{}, assignmentSeed: seed, domains: domains,
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
	view, err := opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	got, available := view.PublisherAttachment(now, deadline)
	if !available || got.NetworkID != network || got.Epoch != 1 || got.Digest != epoch.Digest || !got.NotAfter.Equal(deadline) {
		t.Fatalf("PublisherAttachment = %+v, available=%t", got, available)
	}
	assertPublisherPeer(t, "Introduction", got.Introduction, expected["introduction"])
	assertPublisherPeer(t, "Rendezvous", got.Rendezvous, expected["rendezvous"])
	assertPublisherPeer(t, "Responder", got.Responder, expected["responder"])
}

func TestResolutionViewRejectsIncompleteOrAmbiguousPublisherAttachment(t *testing.T) {
	t.Parallel()
	now := time.Unix(fixtureNow, 0).UTC()
	network := sha256.Sum256([]byte("unavailable-publisher-attachment-state"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xab}, ed25519.SeedSize))
	domains := []string{"initiator", "introduction", "rendezvous", "responder"}
	seed := sha256.Sum256([]byte("unavailable-publisher-attachment-assignment"))
	complete := routeProfileRecords(t, network, seed, domains, now)
	missing := make([]fixtureRecord, 0, len(complete)-1)
	for _, record := range complete {
		domain, err := fixtureAssignmentDomain(network, 1, seed, record.family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if domain != "introduction" {
			missing = append(missing, record)
		}
	}
	ambiguous := append(append([]fixtureRecord(nil), complete...), publisherRecordForDomain(t, network, seed, domains, now, "introduction"))
	for name, records := range map[string][]fixtureRecord{"missing": missing, "ambiguous": ambiguous} {
		t.Run(name, func(t *testing.T) {
			view := acceptedPublisherView(t, network, authority, seed, domains, now, records)
			if got, available := view.PublisherAttachment(now, now.Add(5*time.Second)); available {
				t.Fatalf("PublisherAttachment = %+v, available=true", got)
			}
		})
	}
}

func acceptedPublisherView(t *testing.T, network [32]byte, authority ed25519.PrivateKey, seed [32]byte,
	domains []string, now time.Time, records []fixtureRecord,
) state.ResolutionView {
	t.Helper()
	spec := testEpochSpec{networkID: network, number: 1, validFrom: now.Add(-time.Minute), validUntil: now.Add(time.Hour),
		inputs: recordBytes(records), accepted: records, rejections: map[uint32]uint16{}, assignmentSeed: seed, domains: domains,
		authorities: []ed25519.PrivateKey{authority}, profile: "ardents-interactive-route-v1", version: 1}
	epoch := buildTestEpoch(t, spec)
	opened, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey)},
		Threshold:   1, Now: now, AcceptedProfile: "ardents-interactive-route-v1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	if _, err := opened.Accept(context.Background(), epoch.Raw, spec.inputs, epoch.Materials[:1]); err != nil {
		t.Fatal(err)
	}
	view, err := opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func publisherRecordForDomain(t *testing.T, network, seed [32]byte, domains []string, now time.Time, wanted string) fixtureRecord {
	t.Helper()
	for marker := byte(128); marker != 0; marker++ {
		family := fmt.Sprintf("publisher-family-%d", marker)
		domain, err := fixtureAssignmentDomain(network, 1, seed, family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if domain != wanted {
			continue
		}
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
		return buildTestRecordWithCapability(t, network, sha256.Sum256([]byte{0xe0, marker}), private, family,
			fmt.Sprintf("127.0.0.1:%d", 43000+int(marker)), 1, 2, now.Add(-time.Minute), now.Add(2*time.Hour))
	}
	t.Fatal("could not construct an additional Publisher role candidate")
	return fixtureRecord{}
}

func assertPublisherPeer(t *testing.T, name string, got state.PublisherTransitPeer, expected fixtureRecord) {
	t.Helper()
	if got.NodeID != expected.nodeID || got.PublicKey == [32]byte{} ||
		got.Family != sha256.Sum256([]byte(expected.family)) || got.Endpoint == "" {
		t.Fatalf("%s = %+v", name, got)
	}
}

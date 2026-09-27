package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// A signed profile must join the domain assigned by a genuinely accepted
// Epoch. Refusal leaves no accepted profile; matching input survives reopen.
func TestClosedProfileRoleDomainMustMatchEpochAssignment(t *testing.T) {
	now := time.Now().UTC()
	hour := now.Truncate(time.Hour)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	networkID := sha256.Sum256([]byte("closed role join network"))
	seed := sha256.Sum256([]byte("closed role join seed"))
	domains := []string{"initiator", "rendezvous"}

	// These frozen families select the stated roles under the signed seed.
	buildRecord := func(id byte, family, endpoint string) networkfixture.Record {
		t.Helper()
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{id}, ed25519.SeedSize))
		record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
			NetworkID: networkID, NodeID: [32]byte{id}, Generation: uint64(id),
			ValidFrom: hour.Add(-time.Minute), ValidUntil: hour.Add(2 * time.Hour),
			Family: family, Endpoint: endpoint, Carrier: closedTCPCarrierProfile,
			Capability: 2, Capacity: 3, PrivateKey: key,
		})
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	issuer := buildRecord(1, "closed-role-join-rendezvous-0", "127.0.0.1:4101")
	other := buildRecord(2, "closed-role-join-initiator-0", "127.0.0.1:4102")
	epoch, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{
		NetworkID: networkID, Number: 1, ValidFrom: hour, ValidUntil: hour.Add(2 * time.Hour),
		Inputs: [][]byte{issuer.Raw, other.Raw}, Accepted: []networkfixture.Record{issuer, other},
		AssignmentSeed: seed, Domains: domains, Authorities: []ed25519.PrivateKey{authority},
		Profile: closedRouteProfile, Version: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	config := Config{
		Root: t.TempDir(), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{
			sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey),
		},
		Threshold: 1, Clock: time.Now, ObserveClock: time.Now,
		AcceptedProfile:        closedRouteProfile,
		ClosedProfileAuthority: authority.Public().(ed25519.PublicKey),
	}
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.Accept(context.Background(), epoch.Raw, epoch.Inputs, epoch.Materials[:1]); err != nil {
		t.Fatalf("accept signed Epoch: %v", err)
	}
	issuerEntry := closedProfileNode{nodeID: issuer.NodeID, recordDigest: sha256.Sum256(issuer.Raw),
		domain: 2, subrole: 6, generation: 1}
	otherEntry := closedProfileNode{nodeID: other.NodeID, recordDigest: sha256.Sum256(other.Raw),
		domain: 3, subrole: 1, generation: 2}
	mismatch := testClosedProfileAt(t, authority, networkID, epoch.Digest, epoch.Digest, 1, now,
		[]closedProfileNode{issuerEntry, otherEntry})
	if _, err := parseClosedProfile(mismatch, epoch.Digest, networkID, epoch.Digest, 1,
		authority.Public().(ed25519.PublicKey), now); err != nil {
		t.Fatalf("mismatched signed profile is not structurally valid: %v", err)
	}
	if _, err := store.AcceptClosedProfile(mismatch); err == nil {
		t.Fatal("accepted signed Role Domain contradicting the Epoch assignment")
	}
	if _, err := store.CurrentClosedRoute(); err == nil {
		t.Fatal("mismatched profile exposed a closed route")
	}

	otherEntry.domain = 1
	matching := testClosedProfileAt(t, authority, networkID, epoch.Digest, epoch.Digest, 1, now,
		[]closedProfileNode{issuerEntry, otherEntry})
	view, err := store.AcceptClosedProfile(matching)
	if err != nil || view.Digest != sha256.Sum256(matching) {
		t.Fatalf("accept matching profile: %+v / %v", view, err)
	}
	route, err := store.CurrentClosedRoute()
	if err != nil || route.NodeCount != 2 || route.Nodes[0].RoleDomain != 2 ||
		route.Nodes[1].RoleDomain != 1 {
		t.Fatalf("current joined route: %+v / %v", route, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(config)
	if err != nil {
		t.Fatalf("reopen accepted State/profile: %v", err)
	}
	defer reopened.Close()
	route, err = reopened.CurrentClosedRoute()
	if err != nil || route.NodeCount != 2 || route.Nodes[0].RoleDomain != 2 ||
		route.Nodes[1].RoleDomain != 1 {
		t.Fatalf("reopened joined route: %+v / %v", route, err)
	}
}

package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

// TestClosedProfileRoleDomainMustMatchEpochAssignment proves the F-45 join: a
// correctly signed closed profile whose numeric Role Domain contradicts the
// record family's authenticated Epoch assignment is refused before durable
// acceptance and never exposes a route, while the identical topology that
// agrees with the assignment is accepted. The Epoch here carries the four
// contract role-name domains, so assignedDomain resolves each record to a role
// the profile must mirror; a structurally valid duty that names the wrong role
// is the exact gap the join closes.
func TestClosedProfileRoleDomainMustMatchEpochAssignment(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	generation := sha256.Sum256([]byte("closed profile generation"))
	network := sha256.Sum256([]byte("closed role join network"))
	epochDigest := sha256.Sum256([]byte("closed role join epoch"))
	seed := sha256.Sum256([]byte("closed role join seed"))
	domains := []string{"initiator", "rendezvous"}
	epoch := epochEnvelope{networkID: network, number: 1, assignmentSeed: seed,
		domains: []roleDomain{{id: "initiator"}, {id: "rendezvous"}}}

	// The issuer record must land on rendezvous (Role Domain 2, subrole 6); the
	// second record must land on initiator (Role Domain 1). Family text is
	// free-form, so search names that the Epoch assignment resolves as required.
	searchFamily := func(target string) string {
		t.Helper()
		for attempt := 0; attempt < 4096; attempt++ {
			family := fmt.Sprintf("closed-role-join-%s-%d", target, attempt)
			selected, err := selectEpochDomain(network, 1, seed, family, domains)
			if err != nil {
				t.Fatalf("role assignment: %v", err)
			}
			if selected == target {
				return family
			}
		}
		t.Fatalf("no family resolved to role domain %q", target)
		return ""
	}
	issuer := nodeRecord{raw: []byte("authenticated issuer record"), nodeID: [32]byte{1}, generation: 5,
		family: searchFamily("rendezvous"), carrier: closedTCPCarrierProfile}
	other := nodeRecord{raw: []byte("authenticated other record"), nodeID: [32]byte{2}, generation: 6,
		family: searchFamily("initiator"), carrier: closedTCPCarrierProfile}

	newStore := func() *networkState {
		t.Helper()
		root, err := openDurableRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = root.close() })
		return &networkState{config: config{closedProfileAuthority: authority.Public().(ed25519.PublicKey),
			clock: func() time.Time { return now }, observe: func() time.Time { return now }}, storage: root,
			current: &Snapshot{Generation: fmt.Sprintf("%x", generation), NetworkID: network, Epoch: 9, Digest: epochDigest,
				EpochValidFrom: now.Truncate(time.Hour), ValidUntil: now.Truncate(time.Hour).Add(2 * time.Hour), Profile: closedRouteProfile},
			currentDecision: &candidateDecision{verified: verifiedEpochDecision{epoch: epoch, accepted: []nodeRecord{issuer, other}}}}
	}
	issuerEntry := closedProfileNode{nodeID: issuer.nodeID, recordDigest: sha256.Sum256(issuer.raw), domain: 2, subrole: 6, generation: issuer.generation}

	// A structurally valid, correctly signed profile that names the second Node
	// as a responder (Role Domain 3) contradicts its initiator assignment.
	mismatch := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{issuerEntry,
		{nodeID: other.nodeID, recordDigest: sha256.Sum256(other.raw), domain: 3, subrole: 1, generation: other.generation}})
	if _, err := parseClosedProfile(mismatch, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err != nil {
		t.Fatalf("mismatched profile is not structurally valid: %v", err)
	}
	refused := newStore()
	if _, err := refused.AcceptClosedProfile(mismatch); err == nil {
		t.Fatal("accepted a signed profile whose Role Domain contradicts the Epoch assignment")
	}
	if _, err := refused.CurrentClosedRoute(); err == nil {
		t.Fatal("mismatched Role Domain still exposed a closed route")
	}

	// The identical topology naming the assigned initiator domain is accepted.
	joined := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{issuerEntry,
		{nodeID: other.nodeID, recordDigest: sha256.Sum256(other.raw), domain: 1, subrole: 1, generation: other.generation}})
	store := newStore()
	view, err := store.AcceptClosedProfile(joined)
	if err != nil || view.Digest != sha256.Sum256(joined) {
		t.Fatalf("refused a profile whose Role Domain matches the Epoch assignment: %+v, %v", view, err)
	}
	route, err := store.CurrentClosedRoute()
	if err != nil || route.NodeCount != 2 {
		t.Fatalf("current closed route = %+v, %v", route, err)
	}
	for _, node := range route.Nodes[:route.NodeCount] {
		want := map[[32]byte]uint8{issuer.nodeID: 2, other.nodeID: 1}[node.NodeID]
		if node.RoleDomain != want {
			t.Fatalf("joined route node %x has Role Domain %d, want %d", node.NodeID, node.RoleDomain, want)
		}
	}
}

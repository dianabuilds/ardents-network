package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

func TestAcceptClosedProfilePersistsAndConflictsByArrival(t *testing.T) {
	store, first := closedProfileStoreFixture(t)
	now := store.config.clock()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	generation := sha256.Sum256([]byte("closed profile generation"))
	network, epochDigest := store.current.NetworkID, store.current.Digest
	candidate := store.currentDecision.Candidates[0]
	nodeID := candidate.NodeID
	node := closedProfileNode{nodeID: nodeID, recordDigest: candidate.RecordDigest,
		domain: 2, subrole: 6, generation: candidate.RecordGeneration}
	root := store.storage
	parsed, parseErr := closedprofile.Verify(first, closedprofile.Context{StateGeneration: generation, NetworkID: network, EpochDigest: epochDigest, Epoch: 9, Authority: authority.Public().(ed25519.PublicKey), Now: now})
	if parseErr != nil || !matchesClosedProfileCandidates(parsed, store.currentDecision.Candidates) {
		t.Fatalf("closed profile parser/join = %+v, %v, join=%t", parsed, parseErr, matchesClosedProfileCandidates(parsed, store.currentDecision.Candidates))
	}
	view, err := store.AcceptClosedProfile(first)
	if err != nil || view.Digest != sha256.Sum256(first) {
		t.Fatalf("accept first closed profile = %+v, %v", view, err)
	}
	current, err := store.CurrentClosedProfile()
	if err != nil || current != view {
		t.Fatalf("current closed profile = %+v, %v", current, err)
	}
	route, err := store.CurrentClosedRoute()
	if err != nil || route.Profile != view || route.NodeCount != 1 || route.Nodes[0].NodeID != nodeID ||
		route.Nodes[0].RecordDigest != node.recordDigest || route.Nodes[0].RoleDomain != node.domain ||
		route.Nodes[0].Subrole != node.subrole || route.Nodes[0].DutyGeneration != node.generation {
		t.Fatalf("current closed route = %+v, %v", route, err)
	}
	second := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	if _, err := store.AcceptClosedProfile(second); err == nil {
		t.Fatal("accepted a second closed profile digest")
	}
	if _, err := store.CurrentClosedProfile(); err == nil {
		t.Fatal("returned a closed profile after durable conflict")
	}
	state, _, err := root.LoadClosedProfile(generation)
	if err != nil || state.Conflict != sha256.Sum256(second) {
		t.Fatalf("durable profile conflict = %+v, %v", state, err)
	}
}

func closedProfileStoreFixture(t *testing.T) (*networkState, []byte) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	generation := sha256.Sum256([]byte("closed profile generation"))
	network := sha256.Sum256([]byte("closed profile network"))
	epochDigest := sha256.Sum256([]byte("closed profile epoch"))
	nodeID := sha256.Sum256([]byte("issuer node"))
	recordRaw := []byte("authenticated schema-2 record")
	recordGeneration := uint64(5)
	root, err := openTestDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	store := &networkState{config: config{closedProfileAuthority: authority.Public().(ed25519.PublicKey), clock: func() time.Time { return now }, observe: func() time.Time { return now }}, storage: root,
		current: &Snapshot{Generation: fmt.Sprintf("%x", generation), NetworkID: network, Epoch: 9, Digest: epochDigest,
			EpochValidFrom: now.Truncate(time.Hour), ValidUntil: now.Truncate(time.Hour).Add(2 * time.Hour), Profile: closedRouteProfile},
		currentDecision: &epoch.Decision{Candidates: []epoch.Candidate{{
			NodeID: nodeID, RecordDigest: sha256.Sum256(recordRaw), RecordGeneration: recordGeneration,
			CarrierProfile: closedTCPCarrierProfile, Domain: "rendezvous",
		}}}}
	node := closedProfileNode{nodeID: nodeID, recordDigest: sha256.Sum256(recordRaw), domain: 2, subrole: 6, generation: recordGeneration}
	first := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	return store, first
}

package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestClosedProfileStorePersistsConflictAcrossReopen(t *testing.T) {
	root, err := openDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := []byte("one signed closed profile")
	accepted := sha256.Sum256(profile)
	state := closedProfileState{generation: sha256.Sum256([]byte("generation")), epoch: 7, accepted: accepted}
	if err := root.commitClosedProfile(state, profile); err != nil {
		t.Fatal(err)
	}
	state.conflict = sha256.Sum256([]byte("different signed profile"))
	if err := root.commitClosedProfile(state, profile); err != nil {
		t.Fatal(err)
	}
	if err := root.close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openDurableRoot(root.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	got, raw, err := reopened.loadClosedProfile(state.generation)
	if err != nil || got != state || string(raw) != string(profile) {
		t.Fatalf("reopen closed profile = %+v, %q, %v", got, raw, err)
	}
	nextProfile := []byte("next epoch signed closed profile")
	next := closedProfileState{generation: sha256.Sum256([]byte("next generation")), epoch: 8, accepted: sha256.Sum256(nextProfile)}
	if err := reopened.commitClosedProfile(next, nextProfile); err != nil {
		t.Fatalf("persist successor profile: %v", err)
	}
	if got, raw, err := reopened.loadClosedProfile(next.generation); err != nil || got != next || string(raw) != string(nextProfile) {
		t.Fatalf("load successor profile = %+v, %q, %v", got, raw, err)
	}
}

func TestAcceptClosedProfilePersistsAndConflictsByArrival(t *testing.T) {
	store, first := closedProfileStoreFixture(t)
	now := store.config.clock()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	generation := sha256.Sum256([]byte("closed profile generation"))
	network, epochDigest := store.current.NetworkID, store.current.Digest
	record := store.currentDecision.verified.accepted[0]
	nodeID := record.nodeID
	node := closedProfileNode{nodeID: nodeID, recordDigest: sha256.Sum256(record.raw), domain: 2, subrole: 6, generation: record.generation}
	root := store.storage
	parsed, parseErr := parseClosedProfile(first, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now)
	if parseErr != nil || !matchesClosedProfileCandidates(parsed, []nodeRecord{record}) {
		t.Fatalf("closed profile parser/join = %+v, %v, join=%t", parsed, parseErr, matchesClosedProfileCandidates(parsed, []nodeRecord{record}))
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
	state, _, err := root.loadClosedProfile(generation)
	if err != nil || state.conflict != sha256.Sum256(second) {
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
	record := nodeRecord{raw: []byte("authenticated schema-2 record"), nodeID: nodeID, generation: 5, carrier: closedTCPCarrierProfile}
	root, err := openDurableRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.close() })
	store := &networkState{config: config{closedProfileAuthority: authority.Public().(ed25519.PublicKey), clock: func() time.Time { return now }, observe: func() time.Time { return now }}, storage: root,
		current: &Snapshot{Generation: fmt.Sprintf("%x", generation), NetworkID: network, Epoch: 9, Digest: epochDigest,
			EpochValidFrom: now.Truncate(time.Hour), ValidUntil: now.Truncate(time.Hour).Add(2 * time.Hour), Profile: closedRouteProfile},
		currentDecision: &candidateDecision{verified: verifiedEpochDecision{accepted: []nodeRecord{record}}}}
	node := closedProfileNode{nodeID: nodeID, recordDigest: sha256.Sum256(record.raw), domain: 2, subrole: 6, generation: record.generation}
	first := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	return store, first
}

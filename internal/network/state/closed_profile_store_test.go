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
	defer root.close()
	store := &networkState{config: config{closedProfileAuthority: authority.Public().(ed25519.PublicKey), clock: func() time.Time { return now }}, storage: root,
		current: &Snapshot{Generation: fmt.Sprintf("%x", generation), NetworkID: network, Epoch: 9, Digest: epochDigest,
			EpochValidFrom: now.Truncate(time.Hour), ValidUntil: now.Truncate(time.Hour).Add(2 * time.Hour), Profile: closedRouteProfile},
		currentDecision: &candidateDecision{verified: verifiedEpochDecision{accepted: []nodeRecord{record}}}}
	node := closedProfileNode{nodeID: nodeID, recordDigest: sha256.Sum256(record.raw), domain: 2, subrole: 6, generation: record.generation}
	first := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	parsed, parseErr := parseClosedProfile(first, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now)
	if parseErr != nil || !matchesClosedProfileCandidates(parsed, []nodeRecord{record}) {
		t.Fatalf("closed profile parser/join = %+v, %v, join=%t", parsed, parseErr, matchesClosedProfileCandidates(parsed, []nodeRecord{record}))
	}
	view, err := store.AcceptClosedProfile(first)
	if err != nil || view.Digest != sha256.Sum256(first) {
		t.Fatalf("accept first closed profile = %+v, %v", view, err)
	}
	second := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	if _, err := store.AcceptClosedProfile(second); err == nil {
		t.Fatal("accepted a second closed profile digest")
	}
	state, _, err := root.loadClosedProfile(generation)
	if err != nil || state.conflict != sha256.Sum256(second) {
		t.Fatalf("durable profile conflict = %+v, %v", state, err)
	}
}

//go:build linux

package descriptorhistory

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func floorFixtureID(value byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = value + byte(i)
	}
	return id
}

func floorPrivateRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func floorPublication(t *testing.T, authority ed25519.PrivateKey, network [32]byte, generation uint64, from, until time.Time) (publication.Current, ed25519.PrivateKey) {
	t.Helper()
	public, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(signer) })
	var instance [32]byte
	copy(instance[:], public)
	credential, err := (publication.Credential{InstancePublic: instance,
		Generation: generation, NotBefore: from.Unix(), NotAfter: until.Unix(), NetworkID: network, Capabilities: 3}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	root, err := publication.Open(publication.Config{Root: floorPrivateRoot(t), NetworkID: network, Authority: authority.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	current, err := root.Publish(t.Context(), publication.PublishInput{Credential: credential, InstanceSigner: signer, Acknowledgement: []byte("explicit floor test publication"), At: from})
	if err != nil {
		t.Fatal(err)
	}
	return current, signer
}

func floorDescriptor(t *testing.T, current publication.Current, signer ed25519.PrivateKey, profile, node [32]byte, revision uint64, slot byte, from, until time.Time) []byte {
	t.Helper()
	raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, InstanceSigner: signer, ProfileDigest: profile,
		Introduction: reachability.PrivateIntroduction{Revision: revision, NodeID: node, Slot: floorFixtureID(slot), RecipientKey: floorFixtureID(slot + 1), NotBefore: from, NotAfter: until}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTextDescriptorFloorRejectsRollbackAndRetainsConflicts(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network, profile, node := floorFixtureID(1), floorFixtureID(2), floorFixtureID(3)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	current, signer := floorPublication(t, authority, network, 1, now, now.Add(10*time.Minute))
	history := &History{}
	accept := func(raw []byte, at time.Time) error {
		_, err := history.Accept(raw, current.Credential.Target, network, profile, at)
		return err
	}
	first := floorDescriptor(t, current, signer, profile, node, 1, 10, now, now.Add(300*time.Second))
	second := floorDescriptor(t, current, signer, profile, node, 2, 20, now, now.Add(100*time.Second))
	if err := accept(first, now); err != nil {
		t.Fatal(err)
	}
	if err := accept(second, now); err != nil {
		t.Fatalf("higher revision with shorter expiry: %v", err)
	}
	if err := accept(second, now); err != nil {
		t.Fatalf("exact retry: %v", err)
	}
	if err := accept(first, now); err == nil {
		t.Fatal("lower revision revived")
	}
	if err := accept(first, now.Add(110*time.Second)); err == nil {
		t.Fatal("expired successor revived still-valid predecessor")
	}
	forged := append([]byte(nil), second...)
	forged[len(forged)-1] ^= 1
	if err := accept(forged, now); err == nil {
		t.Fatal("forged signature accepted")
	}
	if err := accept(second, now); err != nil {
		t.Fatalf("forgery poisoned floor: %v", err)
	}
	conflicting := floorDescriptor(t, current, signer, profile, node, 2, 30, now, now.Add(120*time.Second))
	if err := accept(conflicting, now); err == nil {
		t.Fatal("same revision conflict accepted")
	}
	if err := accept(second, now); err == nil {
		t.Fatal("exact retry erased revision conflict")
	}
	third := floorDescriptor(t, current, signer, profile, node, 3, 40, now, now.Add(140*time.Second))
	if err := accept(third, now); err != nil {
		t.Fatalf("higher revision failed to repair revision conflict: %v", err)
	}
	// Caller mutation cannot change the retained hashes or create cache aliases.
	verified, err := history.Accept(third, current.Credential.Target, network, profile, now)
	if err != nil {
		t.Fatal(err)
	}
	clear(verified.Current.Record)
	clear(verified.Descriptor.Publication)
	if err := accept(third, now); err != nil {
		t.Fatalf("returned proof aliases retained state: %v", err)
	}
}

func TestTextDescriptorFloorPublicationConflictRetainsLongestAuthority(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network, profile, node := floorFixtureID(1), floorFixtureID(2), floorFixtureID(3)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	history := &History{}
	first, signer := floorPublication(t, authority, network, 1, now, now.Add(100*time.Second))
	firstRaw := floorDescriptor(t, first, signer, profile, node, 1, 10, now, now.Add(100*time.Second))
	accept := func(raw []byte, at time.Time) error {
		_, err := history.Accept(raw, first.Credential.Target, network, profile, at)
		return err
	}
	if err := accept(firstRaw, now); err != nil {
		t.Fatal(err)
	}
	other, otherSigner := floorPublication(t, authority, network, 1, now, now.Add(200*time.Second))
	otherRaw := floorDescriptor(t, other, otherSigner, profile, node, 2, 20, now, now.Add(150*time.Second))
	if err := accept(otherRaw, now); err == nil {
		t.Fatal("conflicting Instance at same generation accepted")
	}
	higherRevision := floorDescriptor(t, first, signer, profile, node, 3, 30, now, now.Add(90*time.Second))
	if err := accept(higherRevision, now); err == nil {
		t.Fatal("revision repaired publication conflict")
	}
	overlapping, overlapSigner := floorPublication(t, authority, network, 2, now.Add(100*time.Second), now.Add(300*time.Second))
	overlapRaw := floorDescriptor(t, overlapping, overlapSigner, profile, node, 1, 40, now.Add(100*time.Second), now.Add(300*time.Second))
	if err := accept(overlapRaw, now.Add(100*time.Second)); err == nil {
		t.Fatal("successor overlapped longer conflicting authority")
	}
	next, nextSigner := floorPublication(t, authority, network, 3, now.Add(200*time.Second), now.Add(400*time.Second))
	nextRaw := floorDescriptor(t, next, nextSigner, profile, node, 1, 50, now.Add(200*time.Second), now.Add(400*time.Second))
	if err := accept(nextRaw, now.Add(200*time.Second)); err != nil {
		t.Fatalf("non-overlapping successor refused: %v", err)
	}
	if err := accept(overlapRaw, now.Add(210*time.Second)); err == nil {
		t.Fatal("older publication generation revived")
	}
}

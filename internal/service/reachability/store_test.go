package reachability_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestStorePersistsTargetFloorAndRejectsStaleSlot(t *testing.T) {
	t.Parallel()
	fixture := newStoreFixture(t)
	root := t.TempDir()
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	raw := fixture.issue(t, fixture.current, fixture.now.Add(30*time.Second), "first")
	result, err := store.Publish(raw, fixture.now)
	if err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("Publish = %+v, %v", result, err)
	}
	if received, class, lookupErr := store.Lookup(fixture.current.Credential.Target, fixture.now); lookupErr != nil || class != reachability.StoreAlreadyCurrent || string(received) != string(raw) {
		t.Fatalf("Lookup = %q, %q, %v", received, class, lookupErr)
	}
	if result, err = store.Publish(raw, fixture.now); err != nil || result.Class != reachability.StoreAlreadyCurrent {
		t.Fatalf("repeat Publish = %+v, %v", result, err)
	}
	stale := fixture.issue(t, fixture.current, fixture.now.Add(20*time.Second), "stale")
	if result, err = store.Publish(stale, fixture.now); err == nil || result.Class != reachability.StoreStale {
		t.Fatalf("stale slot Publish = %+v, %v", result, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if received, class, lookupErr := reopened.Lookup(fixture.current.Credential.Target, fixture.now); lookupErr != nil || class != reachability.StoreAlreadyCurrent || string(received) != string(raw) {
		t.Fatalf("restarted Lookup = %q, %q, %v", received, class, lookupErr)
	}
}

func TestStorePersistsSameGenerationPublicationConflict(t *testing.T) {
	t.Parallel()
	fixture := newStoreFixture(t)
	root := t.TempDir()
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first := fixture.issue(t, fixture.current, fixture.now.Add(30*time.Second), "first")
	if _, err := store.Publish(first, fixture.now); err != nil {
		t.Fatal(err)
	}
	conflictingCurrent := fixture.publish(t, "different-acknowledgement")
	second := fixture.issue(t, conflictingCurrent, fixture.now.Add(40*time.Second), "second")
	if result, publishErr := store.Publish(second, fixture.now); publishErr == nil || result.Class != reachability.StoreConflicting {
		t.Fatalf("conflicting Publish = %+v, %v", result, publishErr)
	}
	if _, class, lookupErr := store.Lookup(fixture.current.Credential.Target, fixture.now); lookupErr == nil || class != reachability.StoreConflicting {
		t.Fatalf("conflicting Lookup = %q, %v", class, lookupErr)
	}
}

type storeFixture struct {
	now          time.Time
	network      [32]byte
	authority    ed25519.PublicKey
	authorityKey ed25519.PrivateKey
	instance     ed25519.PrivateKey
	credential   publication.Credential
	current      publication.Current
}

func newStoreFixture(t *testing.T) storeFixture {
	t.Helper()
	now := time.Unix(2_000_100_000, 0).UTC()
	network := [32]byte{31}
	authority, authorityKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instance, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var fixedInstance [32]byte
	copy(fixedInstance[:], instancePublic)
	credential, err := (publication.Credential{InstancePublic: fixedInstance, IntroductionHPKEPublic: [32]byte{33}, Generation: 1,
		NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(time.Minute).Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityKey)
	if err != nil {
		t.Fatal(err)
	}
	fixture := storeFixture{now: now, network: network, authority: authority, authorityKey: authorityKey, instance: instance, credential: credential}
	fixture.current = fixture.publish(t, "first-acknowledgement")
	return fixture
}

func (fixture storeFixture) publish(t *testing.T, acknowledgement string) publication.Current {
	t.Helper()
	owner, err := publication.Open(publication.Config{Root: t.TempDir(), NetworkID: fixture.network, Authority: fixture.authority, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	current, err := owner.Publish(context.Background(), publication.PublishInput{Credential: fixture.credential, InstanceSigner: fixture.instance,
		Acknowledgement: []byte(acknowledgement), At: fixture.now})
	if err != nil {
		t.Fatal(err)
	}
	return current
}

func (fixture storeFixture) issue(t *testing.T, current publication.Current, notAfter time.Time, authorization string) []byte {
	t.Helper()
	raw, _, err := reachability.Issue(reachability.IssueInput{Current: current, InstanceSigner: fixture.instance,
		Introduction: reachability.Introduction{StateDigest: [32]byte{34}, Epoch: 9, IntroductionNodeID: [32]byte{35}, RendezvousNodeID: [32]byte{36},
			Reachability: [32]byte{37}, JoinHandle: [32]byte{38}, NotAfter: notAfter, SubmissionAuthorization: []byte(authorization)}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

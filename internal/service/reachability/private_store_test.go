//go:build linux

package reachability_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func privateStoreDescriptor(t *testing.T, fixture storeFixture, current publication.Current, revision uint64, slot byte, expiry time.Time) []byte {
	t.Helper()
	raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, ProfileDigest: [32]byte{71}, InstanceSigner: fixture.instance,
		Introduction: reachability.PrivateIntroduction{Revision: revision, NodeID: [32]byte{72}, Slot: [32]byte{slot}, RecipientKey: [32]byte{74, slot},
			NotBefore: fixture.now, NotAfter: expiry}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPrivateStorePersistsRevisionConflictAndNeverReturnsLowerRevision(t *testing.T) {
	fixture := newStoreFixture(t)
	root := t.TempDir()
	profile, target := [32]byte{71}, fixture.current.Credential.Target
	open := func() *reachability.Store {
		store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = store.Close() })
		return store
	}
	store := open()
	first := privateStoreDescriptor(t, fixture, fixture.current, 1, 73, fixture.now.Add(40*time.Second))
	if result, err := store.PublishPrivate(first, profile, fixture.now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("initial private publication: %+v, %v", result, err)
	}
	// Revision, rather than later expiry, orders a legitimate refresh.
	second := privateStoreDescriptor(t, fixture, fixture.current, 2, 75, fixture.now.Add(30*time.Second))
	if result, err := store.PublishPrivate(second, profile, fixture.now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("higher revision with shorter lifetime: %+v, %v", result, err)
	}
	if result, err := store.PublishPrivate(second, profile, fixture.now); err != nil || result.Class != reachability.StoreAlreadyCurrent {
		t.Fatalf("exact retransmission: %+v, %v", result, err)
	}
	if result, err := store.PublishPrivate(first, profile, fixture.now); err == nil || result.Class != reachability.StoreStale {
		t.Fatalf("lower revision returned: %+v, %v", result, err)
	}
	if raw, _, err := store.LookupPrivate(target, profile, fixture.now); err != nil || !bytes.Equal(raw, second) {
		t.Fatal("lookup did not retain exact latest proof")
	}
	conflict := privateStoreDescriptor(t, fixture, fixture.current, 2, 76, fixture.now.Add(35*time.Second))
	if result, err := store.PublishPrivate(conflict, profile, fixture.now); err == nil || result.Class != reachability.StoreConflicting {
		t.Fatalf("same revision conflict: %+v, %v", result, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = open()
	if _, class, err := store.LookupPrivate(target, profile, fixture.now); err == nil || class != reachability.StoreConflicting {
		t.Fatalf("restart lost revision conflict: %v, %v", class, err)
	}
	if _, err := store.PublishPrivate(second, profile, fixture.now); err == nil {
		t.Fatal("exact old proof erased conflict")
	}
	third := privateStoreDescriptor(t, fixture, fixture.current, 3, 77, fixture.now.Add(25*time.Second))
	if result, err := store.PublishPrivate(third, profile, fixture.now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("strictly higher valid revision: %+v, %v", result, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = open()
	if result, err := store.PublishPrivate(conflict, profile, fixture.now); err == nil || result.Class != reachability.StoreStale {
		t.Fatalf("restart lost high revision floor: %+v, %v", result, err)
	}
	if _, _, err := store.LookupPrivate(target, profile, fixture.now.Add(26*time.Second)); err == nil {
		t.Fatal("expired highest revision revived a live predecessor")
	}
}

func TestPrivateStoreKeepsPublicationConflictSeparateFromRevision(t *testing.T) {
	fixture := newStoreFixture(t)
	root, profile := t.TempDir(), [32]byte{71}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	first := privateStoreDescriptor(t, fixture, fixture.current, 1, 73, fixture.now.Add(30*time.Second))
	if _, err := store.PublishPrivate(first, profile, fixture.now); err != nil {
		t.Fatal(err)
	}
	conflicting := fixture.publish(t, "conflicting-publication-proof")
	changed := privateStoreDescriptor(t, fixture, conflicting, 2, 74, fixture.now.Add(40*time.Second))
	if result, err := store.PublishPrivate(changed, profile, fixture.now); err == nil || result.Class != reachability.StoreConflicting {
		t.Fatalf("publication conflict became revision refresh: %+v, %v", result, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	high := privateStoreDescriptor(t, fixture, fixture.current, 100, 75, fixture.now.Add(30*time.Second))
	if _, err := store.PublishPrivate(high, profile, fixture.now); err == nil {
		t.Fatal("revision increment repaired conflicting publication generation")
	}
}

func TestPrivateStoreRefusesWrongProfileLegacyDowngradeAndFailedCommit(t *testing.T) {
	fixture := newStoreFixture(t)
	root, profile := t.TempDir(), [32]byte{71}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	raw := privateStoreDescriptor(t, fixture, fixture.current, 1, 73, fixture.now.Add(30*time.Second))
	if _, err := store.PublishPrivate(raw, [32]byte{99}, fixture.now); err == nil {
		t.Fatal("wrong current profile accepted")
	}
	if _, err := store.Publish(raw, fixture.now); err == nil {
		t.Fatal("legacy publication accepted private Descriptor")
	}
	path := filepath.Join(root, "records", fmt.Sprintf("%x", fixture.current.Credential.Target))
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PublishPrivate(raw, profile, fixture.now); err == nil {
		t.Fatal("failed durable commit acknowledged")
	}
	if _, _, err := store.LookupPrivate(fixture.current.Credential.Target, profile, fixture.now); err == nil {
		t.Fatal("failed commit changed in-memory floor")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.PublishPrivate(raw, profile, fixture.now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Lookup(fixture.current.Credential.Target, fixture.now); err == nil {
		t.Fatal("legacy lookup exposed private Descriptor")
	}
	if _, _, err := store.LookupPrivate(fixture.current.Credential.Target, [32]byte{99}, fixture.now); err == nil {
		t.Fatal("wrong-profile lookup accepted")
	}
	legacy := fixture.issue(t, fixture.current, fixture.now.Add(40*time.Second), "legacy-slot")
	if _, err := store.Publish(legacy, fixture.now); err == nil {
		t.Fatal("legacy format erased private revision floor")
	}
}

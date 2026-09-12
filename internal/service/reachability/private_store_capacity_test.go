//go:build linux

package reachability_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestPrivateStoreBoundsTargetsBeforeAcknowledgement(t *testing.T) {
	fixture := newStoreFixture(t)
	root, profile := t.TempDir(), [32]byte{71}
	config := reachability.StoreConfig{Root: root, NetworkID: fixture.network}
	store, err := reachability.OpenStore(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for index := 0; index < 129; index++ {
		candidate := fixture
		if index != 0 {
			candidate = newStoreFixture(t)
		}
		raw := privateStoreDescriptor(t, candidate, candidate.current, 1, 73, candidate.now.Add(30*time.Second))
		result, err := store.PublishPrivate(raw, profile, fixture.now)
		if index < 128 && (err != nil || result.Class != reachability.StoreAccepted) {
			t.Fatalf("Target %d: %v %v", index, result, err)
		}
		if index == 128 && err == nil {
			t.Fatal("129th Target acknowledged beyond recoverable capacity")
		}
	}
	updated := privateStoreDescriptor(t, fixture, fixture.current, 2, 74, fixture.now.Add(30*time.Second))
	if _, err := store.PublishPrivate(updated, profile, fixture.now); err != nil {
		t.Fatalf("full Store refused existing Target update: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = reachability.OpenStore(config)
	if err != nil {
		t.Fatalf("accepted population cannot reopen: %v", err)
	}
	raw, _, err := store.LookupPrivate(fixture.current.Credential.Target, profile, fixture.now)
	if err != nil || !bytes.Equal(raw, updated) {
		t.Fatal("capacity refusal lost existing floor")
	}
}

func TestPrivateStoreDoesNotRecreateLostInitializedRecords(t *testing.T) {
	for _, lost := range []string{"records", ".ardents-reachability-store-v1"} {
		t.Run(lost, func(t *testing.T) {
			fixture := newStoreFixture(t)
			root := t.TempDir()
			config := reachability.StoreConfig{Root: root, NetworkID: fixture.network}
			store, err := reachability.OpenStore(config)
			if err != nil {
				t.Fatal(err)
			}
			raw := privateStoreDescriptor(t, fixture, fixture.current, 1, 73, fixture.now.Add(30*time.Second))
			if _, err := store.PublishPrivate(raw, [32]byte{71}, fixture.now); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			// Retain actual bytes outside this Store, modelling missing durable state.
			if err := os.Rename(filepath.Join(root, lost), filepath.Join(t.TempDir(), "retained")); err != nil {
				t.Fatal(err)
			}
			reopened, err := reachability.OpenStore(config)
			if err == nil {
				_ = reopened.Close()
				t.Fatal("initialized Store recreated missing floor state")
			}
			if _, err := os.Lstat(filepath.Join(root, lost)); !os.IsNotExist(err) {
				t.Fatal("failed reopening recreated missing state")
			}
		})
	}
}

//go:build linux

package reachability_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestPrivateStoreFailedConflictCommitTerminalizesOwner(t *testing.T) {
	for _, kind := range []string{"revision", "publication"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newStoreFixture(t)
			root, profile := t.TempDir(), [32]byte{71}
			store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			first := privateStoreDescriptor(t, fixture, fixture.current, 1, 73, fixture.now.Add(30*time.Second))
			if _, err := store.PublishPrivate(first, profile, fixture.now); err != nil {
				t.Fatal(err)
			}
			current := fixture.current
			if kind == "publication" {
				current = fixture.publish(t, "different-publication")
			}
			conflict := privateStoreDescriptor(t, fixture, current, 1, 74, fixture.now.Add(30*time.Second))
			records, retained := filepath.Join(root, "records"), filepath.Join(root, "retained-records")
			if err := os.Rename(records, retained); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(records, []byte("storage unavailable"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.PublishPrivate(conflict, profile, fixture.now); err == nil {
				t.Fatal("failed conflict write acknowledged")
			}
			// Restore the filesystem, but not a successful commit. The same
			// owner must never resume accepting from an ambiguous old map.
			if err := os.Remove(records); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(retained, records); err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.LookupPrivate(fixture.current.Credential.Target, profile, fixture.now); err == nil {
				t.Fatal("failed conflict persistence left old Target available")
			}
			if _, err := store.PublishPrivate(first, profile, fixture.now); err == nil {
				t.Fatal("restored filesystem silently revived failed owner")
			}
		})
	}
}

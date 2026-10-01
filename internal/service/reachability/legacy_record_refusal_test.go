//go:build linux

package reachability_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// An unsupported stored-record envelope refuses the whole root. Its bytes
// remain untouched, and no older Descriptor decoder is needed.

func seedLegacyRecord(t *testing.T, root string) (string, []byte) {
	t.Helper()
	path := filepath.Join(root, "records", strings.Repeat("ab", 32))
	payload := []byte{byte(1), byte(0), 'r', 'e', 't', 'i', 'r', 'e', 'd'}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, payload
}

func TestOpenStoreRefusesRetiredLegacyRecord(t *testing.T) {
	fixture := newStoreFixture(t)
	root := t.TempDir()
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path, payload := seedLegacyRecord(t, root)
	_, err = reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err == nil {
		t.Fatal("OpenStore accepted an old stored record")
	}
	retained, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(retained, payload) {
		t.Fatalf("refused legacy record bytes changed: %v", readErr)
	}
}

func TestOpenStoreRefusesOldRootBeforeCreatingLease(t *testing.T) {
	fixture := newStoreFixture(t)
	root := t.TempDir()
	oldMarker := []byte("ardents-reachability-store-v1\n")
	oldPath := filepath.Join(root, ".ardents-reachability-store-v1")
	if err := os.WriteFile(oldPath, oldMarker, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network}); err == nil {
		t.Fatal("old reachability root was accepted")
	}
	if _, err := os.Lstat(filepath.Join(root, ".ardents-reachability-store-lock")); !os.IsNotExist(err) {
		t.Fatalf("old root gained a lease: %v", err)
	}
	retained, err := os.ReadFile(oldPath)
	if err != nil || !bytes.Equal(retained, oldMarker) {
		t.Fatalf("old root marker changed: %v", err)
	}
}

func TestOpenStoreRefusesWholeRootMixingLegacyAndPrivateRecords(t *testing.T) {
	fixture := newStoreFixture(t)
	root := t.TempDir()
	profile := [32]byte{71}
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	valid := privateStoreDescriptor(t, fixture, fixture.current, 1, 73, fixture.now.Add(40*time.Second))
	if result, err := store.PublishPrivate(valid, profile, fixture.now); err != nil || result.Class != reachability.StoreAccepted {
		_ = store.Close()
		t.Fatalf("initial private publication: %+v, %v", result, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path, _ := seedLegacyRecord(t, root)
	reopen := func() error {
		_, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
		return err
	}
	if err := reopen(); err == nil {
		t.Fatal("mixed-root OpenStore accepted an old stored record")
	}
	// The refusal is the only obstacle: removing the retired bytes (an
	// explicit operator decision, never a silent Store action) reopens the
	// root with its valid private floor intact.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	reopened, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, class, err := reopened.LookupPrivate(fixture.current.Credential.Target, profile, fixture.now); err != nil || class != reachability.StoreAlreadyCurrent {
		t.Fatalf("valid private floor lost after legacy removal: %v, %v", class, err)
	}
}

//go:build linux

package reachability_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// ADR-0109 (F-32): the retired generation-2 stored-record envelope is
// recognized only to refuse it. The v1/v2 decode grammar is deleted; a root
// holding a legacy record refuses to open as a whole, the historical bytes
// stay on disk untouched, and the typed sentinel names the exact cause.

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
	if !errors.Is(err, reachability.ErrLegacyRecord) {
		t.Fatalf("OpenStore over a legacy record = %v, want ErrLegacyRecord", err)
	}
	retained, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(retained, payload) {
		t.Fatalf("refused legacy record bytes changed: %v", readErr)
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
	if err := reopen(); !errors.Is(err, reachability.ErrLegacyRecord) {
		t.Fatalf("mixed-root OpenStore = %v, want ErrLegacyRecord", err)
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

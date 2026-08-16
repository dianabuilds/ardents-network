package localroles_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/localroles"
)

func TestOpenDoesNotClaimAnUnrelatedDirectory(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "local-roles")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(path, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := localroles.Open(localroles.Config{Root: root, Clock: time.Now, Create: true}); err == nil {
		t.Fatal("unrelated directory was claimed as local role state")
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "unrelated" {
		t.Fatalf("unrelated directory changed: %q, %v", raw, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Fatalf("unrelated directory entries changed: %v, %v", entries, err)
	}
}

func TestStoreRetainsCurrentConflictTruthAcrossRestart(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	producer := [32]byte{1}
	identity, family := [32]byte{2}, [32]byte{3}
	if err := store.Replace(producer, []localroles.Duty{{Identity: identity, Family: family,
		Class: "route-rendezvous", State: "live", NotAfter: now.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if conflict, err := store.Conflict(identity, [32]byte{}); err != nil || !conflict {
		t.Fatalf("identity conflict = %v, %v", conflict, err)
	}
	if conflict, err := store.Conflict([32]byte{}, family); err != nil || !conflict {
		t.Fatalf("family conflict = %v, %v", conflict, err)
	}
}

func TestStoreRequiresInitializationAndOneExclusiveOwner(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	config := localroles.Config{Root: root, Clock: func() time.Time { return now }}
	if _, err := localroles.Open(config); err == nil {
		t.Fatal("uninitialized local role root was accepted")
	}
	config.Create = true
	first, err := localroles.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := localroles.Open(config); err == nil {
		t.Fatal("second local role owner acquired the same root")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	config.Create = false
	second, err := localroles.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
}

func TestOverBoundReplacementChangesNoConflictTruth(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	producer := [32]byte{1}
	retained := localroles.Duty{Identity: [32]byte{2}, Family: [32]byte{3},
		Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)}
	if err := store.Replace(producer, []localroles.Duty{retained}); err != nil {
		t.Fatal(err)
	}
	duties := make([]localroles.Duty, 33)
	for index := range duties {
		duties[index] = localroles.Duty{Identity: [32]byte{byte(index + 1)}, Family: [32]byte{byte(index + 2)},
			Class: "route-interior", State: "live", NotAfter: now.Add(time.Hour)}
	}
	if err := store.Replace(producer, duties); err == nil {
		t.Fatal("over-bound duty replacement succeeded")
	}
	if conflict, err := store.Conflict(retained.Identity, retained.Family); err != nil || !conflict {
		t.Fatalf("retained conflict after rejection = %v, %v", conflict, err)
	}
}

func TestOrdinaryInitiatorDutyDoesNotConflict(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	identity, family := [32]byte{4}, [32]byte{5}
	if err := store.Replace([32]byte{6}, []localroles.Duty{{Identity: identity, Family: family,
		Class: "ordinary-initiator", State: "live", NotAfter: now.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if conflict, err := store.Conflict(identity, family); err != nil || conflict {
		t.Fatalf("ordinary Initiator conflict = %v, %v", conflict, err)
	}
}

func TestExpiredDutyIsIgnoredAndPurgedByNextWrite(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	clock := func() time.Time { return now }
	store, err := localroles.Open(localroles.Config{Root: root, Clock: clock, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	identity, family := [32]byte{7}, [32]byte{8}
	if err := store.Replace([32]byte{9}, []localroles.Duty{{Identity: identity, Family: family,
		Class: "node-duty", State: "live", NotAfter: now.Add(time.Second)}}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	if conflict, err := store.Conflict(identity, family); err != nil || conflict {
		t.Fatalf("expired duty conflict = %v, %v", conflict, err)
	}
	if err := store.Replace([32]byte{10}, []localroles.Duty{{Identity: [32]byte{11}, Family: [32]byte{12},
		Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = localroles.Open(localroles.Config{Root: root, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if conflict, err := store.Conflict(identity, family); err != nil || conflict {
		t.Fatalf("purged duty conflict after restart = %v, %v", conflict, err)
	}
}

func TestConflictingProducerIsRejectedAtomically(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	identity, firstFamily := [32]byte{13}, [32]byte{14}
	first := localroles.Duty{Identity: identity, Family: firstFamily,
		Class: "node-duty", State: "live", NotAfter: now.Add(time.Hour)}
	if err := store.Replace([32]byte{15}, []localroles.Duty{first}); err != nil {
		t.Fatal(err)
	}
	second := localroles.Duty{Identity: identity, Family: [32]byte{16},
		Class: "route-rendezvous", State: "live", NotAfter: now.Add(time.Hour)}
	if err := store.Replace([32]byte{17}, []localroles.Duty{second}); err == nil {
		t.Fatal("conflicting producer was retained")
	}
	if conflict, err := store.Conflict(identity, firstFamily); err != nil || !conflict {
		t.Fatalf("original duty after rejection = %v, %v", conflict, err)
	}
}

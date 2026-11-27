//go:build linux || darwin || windows

package store_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	descriptorstore "github.com/dianabuilds/ardents-network/internal/domain/reachability/store"
	"github.com/dianabuilds/ardents-network/tests/descriptorfixture"
)

// These tests require the actual Unix-family/Windows lease and directory-sync adapter.
// They prove the bounded Descriptor repair's Store half: an uncertain stage is
// removed only under the exclusive lease after every committed floor validates,
// a missing root is never initialized, committed floors are never reset or
// evicted, staging is never promoted, and a held lease or invalid history
// refuses without changing bytes. The dead-unit manager/cgroup proofs belong to
// the repair adapter, not this mechanism; no publication or runtime authority is
// granted here.
func TestRepairRetainedStoreStageRemovesUncertainStageUnderLeaseAndPreservesFloors(t *testing.T) {
	raw, target, network, profile, at := descriptorfixture.Fixed(3, 0x11)
	root := t.TempDir()
	config := descriptorstore.Config{Root: root, Network: network}
	store, err := descriptorstore.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, "records", storeTargetNameForTest(target))
	committed, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	// A crash mid-Publish leaves the reserved staging name. Its contents are
	// never a committed proof, even when they are complete valid envelope bytes.
	// A distinct authority seed gives the staged Target a different name from the
	// committed floor, so the no-promotion assertion is meaningful.
	foreignRaw, foreignTarget, _, _, _ := descriptorfixture.Fixed(3, 0x99)
	stage := filepath.Join(root, "records", ".record-stage")
	if err := os.WriteFile(stage, append([]byte{3, 0}, foreignRaw...), 0600); err != nil {
		t.Fatal(err)
	}
	if retained, err := descriptorstore.OpenRetained(config); err == nil {
		_ = retained.Close()
		t.Fatal("retained opening adopted the uncertain stage")
	}
	if err := descriptorstore.RepairStage(config); err != nil {
		t.Fatal("bounded stage repair refused", err)
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatal("uncertain stage retained after repair", err)
	}
	after, err := os.ReadFile(record)
	if err != nil || !bytes.Equal(committed, after) {
		t.Fatal("repair changed the committed floor", err)
	}
	// Staging is removed, never promoted: the staged Target has no record.
	if _, err := os.Lstat(filepath.Join(root, "records", storeTargetNameForTest(foreignTarget))); !os.IsNotExist(err) {
		t.Fatal("repair promoted uncommitted staging", err)
	}
	retained, err := descriptorstore.OpenRetained(config)
	if err != nil {
		t.Fatal("retained opening refused after bounded repair", err)
	}
	got, outcome, lookupErr := retained.Lookup(target, profile, at)
	closeErr := retained.Close()
	if lookupErr != nil || closeErr != nil || outcome != reachability.Accepted || !bytes.Equal(raw, got) {
		t.Fatal("committed publication lost after repair", outcome, lookupErr, closeErr)
	}
}

func TestRepairRetainedStoreStageNoStageIsValidatingNoOp(t *testing.T) {
	raw, target, network, profile, at := descriptorfixture.Fixed(3, 0x11)
	root := t.TempDir()
	config := descriptorstore.Config{Root: root, Network: network}
	store, err := descriptorstore.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	before := storeRootBytesForTest(t, root)
	if err := descriptorstore.RepairStage(config); err != nil {
		t.Fatal("no-stage repair refused", err)
	}
	if after := storeRootBytesForTest(t, root); !bytes.Equal(before, after) {
		// Compare the full byte inventory; map order is irrelevant.
		t.Fatal("no-op repair changed retained bytes")
	}
	retained, err := descriptorstore.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	got, outcome, lookupErr := retained.Lookup(target, profile, at)
	closeErr := retained.Close()
	if lookupErr != nil || closeErr != nil || outcome != reachability.Accepted || !bytes.Equal(raw, got) {
		t.Fatal("no-op repair lost the committed floor", outcome, lookupErr, closeErr)
	}
}

func TestRepairRetainedStoreStageMissingRootNeverInitializes(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "missing")
	config := descriptorstore.Config{Root: root, Network: [32]byte{1}}
	if err := descriptorstore.RepairStage(config); err == nil {
		t.Fatal("missing root repaired")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("repair initialized a missing root", err)
	}
	for _, invalid := range []descriptorstore.Config{{Root: "", Network: [32]byte{1}}, {Root: root, Network: [32]byte{}}} {
		if err := descriptorstore.RepairStage(invalid); err == nil {
			t.Fatal("invalid repair configuration accepted")
		}
	}
}

func TestRepairRetainedStoreStageHeldLeaseRefusesWithoutChanges(t *testing.T) {
	raw, _, network, profile, at := descriptorfixture.Fixed(3, 0x11)
	root := t.TempDir()
	config := descriptorstore.Config{Root: root, Network: network}
	store, err := descriptorstore.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "records", ".record-stage")
	staged := []byte("original uncertain unpublished write")
	if err := os.WriteFile(stage, staged, 0600); err != nil {
		t.Fatal(err)
	}
	before := storeRootBytesForTest(t, root)
	// The live Store holds the exclusive lease; repair must refuse rather than
	// reacquire or share it.
	if err := descriptorstore.RepairStage(config); err == nil {
		t.Fatal("repair acquired a held Store lease")
	}
	if after := storeRootBytesForTest(t, root); !bytes.Equal(before, after) {
		t.Fatal("held-lease refusal changed retained bytes")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := descriptorstore.RepairStage(config); err != nil {
		t.Fatal("repair refused after the original lease released", err)
	}
	if _, err := os.Lstat(stage); !os.IsNotExist(err) {
		t.Fatal("stage retained after released-lease repair", err)
	}
}

func TestRepairRetainedStoreStageInvalidHistoryRefusesAndPreservesEvidence(t *testing.T) {
	for _, fault := range []string{"corrupt-floor", "foreign-root-entry", "missing-marker"} {
		t.Run(fault, func(t *testing.T) {
			raw, target, network, profile, at := descriptorfixture.Fixed(3, 0x11)
			root := t.TempDir()
			config := descriptorstore.Config{Root: root, Network: network}
			store, err := descriptorstore.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Publish(raw, profile, at, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			stage := filepath.Join(root, "records", ".record-stage")
			staged := []byte("original uncertain unpublished write")
			if err := os.WriteFile(stage, staged, 0600); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "corrupt-floor":
				record := filepath.Join(root, "records", storeTargetNameForTest(target))
				if err := os.WriteFile(record, []byte("invalid committed bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign-root-entry":
				if err := os.WriteFile(filepath.Join(root, "foreign"), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing-marker":
				if err := os.Remove(filepath.Join(root, ".ardents-reachability-store-v3")); err != nil {
					t.Fatal(err)
				}
			}
			before := storeRootBytesForTest(t, root)
			if err := descriptorstore.RepairStage(config); err == nil {
				t.Fatal("invalid retained history repaired", fault)
			}
			if after := storeRootBytesForTest(t, root); !bytes.Equal(before, after) {
				t.Fatal("refusal changed retained evidence", fault)
			}
			// Every committed floor validates before staging is touched: the
			// uncertain stage survives a floor refusal untouched.
			if fault == "corrupt-floor" {
				after, err := os.ReadFile(stage)
				if err != nil || !bytes.Equal(staged, after) {
					t.Fatal("floor refusal removed the uncertain stage", err)
				}
			}
		})
	}
}

func storeTargetNameForTest(target [32]byte) string {
	const hexDigits = "0123456789abcdef"
	name := make([]byte, 64)
	for index, value := range target {
		name[index*2] = hexDigits[value>>4]
		name[index*2+1] = hexDigits[value&0x0f]
	}
	return string(name)
}

func storeRootBytesForTest(t *testing.T, root string) []byte {
	t.Helper()
	var inventory []byte
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		inventory = append(inventory, relative...)
		inventory = append(inventory, 0)
		if entry.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		inventory = append(inventory, raw...)
		inventory = append(inventory, 0)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

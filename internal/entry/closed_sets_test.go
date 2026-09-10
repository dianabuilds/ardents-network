//go:build linux

package entry

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func closedEntryFixture(t *testing.T) (ClosedSetConfig, *ClosedSetView) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	view := &ClosedSetView{NetworkID: [32]byte{1}, Now: now}
	for index := byte(2); index < 6; index++ {
		view.Candidates = append(view.Candidates, ClosedSetMember{NodeID: [32]byte{index}, PublicKey: [32]byte{index + 10}, FamilyID: [32]byte{index + 20}, RecordDigest: [32]byte{index + 30},
			DutyGeneration: uint64(index), Domain: 1, NotAfter: now.Add(12 * time.Hour)})
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return ClosedSetConfig{Root: root, NetworkID: view.NetworkID, Current: func() (ClosedSetView, error) { return *view, nil }}, view
}

func TestClosedEntrySetPersistsChoicesAndNeverRefillsOnFailure(t *testing.T) {
	config, view := closedEntryFixture(t)
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := owner.Members(1)
	if err != nil {
		t.Fatal(err)
	}
	if selected[0].NodeID == selected[1].NodeID || selected[0].FamilyID == selected[1].FamilyID {
		t.Fatal("conflicting selected pair")
	}
	if second, err := OpenClosedSets(config); err == nil {
		_ = second.Close()
		t.Fatal("two owners claimed Entry root")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	repeated, err := owner.Members(1)
	if err != nil || selected != repeated {
		t.Fatalf("restart rotated set: %v", err)
	}
	var current []ClosedSetMember
	for _, candidate := range view.Candidates {
		if candidate.NodeID != selected[0].NodeID {
			current = append(current, candidate)
		}
	}
	view.Candidates = current
	if _, err := owner.CurrentMember(1, 0); err == nil {
		t.Fatal("withdrawn member still current")
	}
	if alternative, err := owner.CurrentMember(1, 1); err != nil || alternative != selected[1] {
		t.Fatalf("retained alternative lost: %v", err)
	}
	repeated, err = owner.Members(1)
	if err != nil || selected != repeated {
		t.Fatal("failure sampled replacement")
	}
}

func TestClosedEntrySetConcurrentActivationCommitsOnePair(t *testing.T) {
	config, _ := closedEntryFixture(t)
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	results := make(chan [2]ClosedSetMember, 16)
	failures := make(chan error, 16)
	var callers sync.WaitGroup
	for range 16 {
		callers.Go(func() { members, err := owner.Members(1); results <- members; failures <- err })
	}
	callers.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := <-results
	for selected := range results {
		if selected != first {
			t.Fatal("concurrent callers drew different Entry sets")
		}
	}
	if owner.state.Generation != 2 {
		t.Fatal("activation committed more than one selection")
	}
}

func TestClosedEntrySetHonorsLifetimeAndTimeRegression(t *testing.T) {
	config, view := closedEntryFixture(t)
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Members(1); err != nil {
		t.Fatal(err)
	}
	original := owner.state.Sets[0]
	view.Now = view.Now.Add(-time.Second)
	if _, err := owner.Members(1); err == nil {
		t.Fatal("regressed clock accepted")
	}
	view.Now = original.NotAfter
	if _, err := owner.CurrentMember(1, 0); err == nil {
		t.Fatal("expired set still current")
	}
	if _, err := owner.Members(1); err != nil {
		t.Fatal(err)
	}
	if owner.state.Generation != 3 || owner.state.Sets[0].Chosen != view.Now {
		t.Fatal("expired set did not commit fresh selection")
	}
}

func TestClosedEntrySetReconcilesPointerWithoutResettingFloor(t *testing.T) {
	for _, fault := range []string{"old-pointer", "missing-pointer", "missing-floor", "foreign-network"} {
		t.Run(fault, func(t *testing.T) {
			config, _ := closedEntryFixture(t)
			owner, err := OpenClosedSets(config)
			if err != nil {
				t.Fatal(err)
			}
			selected, err := owner.Members(1)
			if err != nil {
				t.Fatal(err)
			}
			previous := owner.state.Previous
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "old-pointer":
				if err := replaceCurrent(config.Root, previous); err != nil {
					t.Fatal(err)
				}
			case "missing-pointer":
				if err := os.Remove(filepath.Join(config.Root, "current")); err != nil {
					t.Fatal(err)
				}
			case "missing-floor":
				if err := os.Remove(filepath.Join(config.Root, "watermark")); err != nil {
					t.Fatal(err)
				}
			case "foreign-network":
				config.NetworkID[0]++
			}
			owner, err = OpenClosedSets(config)
			if fault == "missing-floor" || fault == "foreign-network" {
				if err == nil {
					_ = owner.Close()
					t.Fatal("ambiguous authority reset")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			repeated, err := owner.Members(1)
			if err != nil || repeated != selected {
				t.Fatal("pointer repair rotated set")
			}
		})
	}
}

func TestClosedEntrySetRefusesConflictsAndIncompleteCreation(t *testing.T) {
	config, view := closedEntryFixture(t)
	view.Candidates = view.Candidates[:2]
	view.Candidates[1].FamilyID = view.Candidates[0].FamilyID
	owner, err := OpenClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Members(1); err == nil {
		t.Fatal("same-family pair accepted")
	}
	if owner.state.Generation != 1 {
		t.Fatal("failed selection consumed generation")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	config.Root = t.TempDir()
	if err := os.Chmod(config.Root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(filepath.Join(config.Root, closedSetMarker), []byte("ardents-entry-set-v1\n")); err != nil {
		t.Fatal(err)
	}
	if owner, err := OpenClosedSets(config); err == nil {
		_ = owner.Close()
		t.Fatal("interrupted initial claim silently reset")
	}
}

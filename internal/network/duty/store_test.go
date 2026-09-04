package duty_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
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
	marker, err := os.ReadFile(filepath.Join(root, ".ardents-local-roles-v1"))
	if err != nil || string(marker) != "ardents-local-roles-v1\n" {
		t.Fatalf("D02 ownership marker changed: %q, %v", marker, err)
	}
	current, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
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
	retainedCurrent, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil || string(retainedCurrent) != string(current) {
		t.Fatalf("D02 current generation reset: %q, %q, %v", current, retainedCurrent, err)
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
	duties := make([]localroles.Duty, 65)
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

func TestSpendTransitGrantRejectsReplayAfterRestart(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	clock := func() time.Time { return now }
	store, err := localroles.Open(localroles.Config{Root: root, Clock: clock, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	nodeID, grantID := [32]byte{21}, [32]byte{22}
	notAfter := now.Add(time.Hour)
	if err := store.SpendTransitGrant(nodeID, grantID, notAfter); err != nil {
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
	if err := store.SpendTransitGrant(nodeID, grantID, notAfter); err == nil {
		t.Fatal("spent transit grant was accepted after restart")
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
	if err := store.Replace([32]byte{17}, []localroles.Duty{second}); !errors.Is(err, localroles.ErrLocalRoleConflict) {
		t.Fatalf("conflicting producer returned %v, want ErrLocalRoleConflict", err)
	}
	if conflict, err := store.Conflict(identity, firstFamily); err != nil || !conflict {
		t.Fatalf("original duty after rejection = %v, %v", conflict, err)
	}
}

// GAP-7-T1: insert 64 direct-source duties (one full per-Replace batch) and
// confirm the Replace succeeds. The per-store cap in validRecords and the
// installation-wide direct-source cap are both 64, so a single 64-batch lands
// exactly at the new ceiling.
func TestInstallationDirectSourceCapAcceptsExactlyAtCap(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	batchDuties := make([]localroles.Duty, 0, 64)
	for offset := 0; offset < 64; offset++ {
		identity := [32]byte{byte(offset + 1)}
		family := [32]byte{byte(offset + 101)}
		batchDuties = append(batchDuties, localroles.Duty{Identity: identity, Family: family,
			Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)})
	}
	if err := store.Replace([32]byte{200}, batchDuties); err != nil {
		t.Fatalf("at-cap Replace: %v", err)
	}
}

// GAP-7-T2: a Replace that would push the direct-source count above 64 must
// return ErrInstallationSourceExhausted. The seed batch establishes 64
// direct-source duties; the second Replace from a different producer would
// grow the count to 128, which exceeds the cap and must surface as the
// sentinel (the per-store cap in validRecords is a separate constraint and
// is checked after the direct-source cap).
func TestInstallationDirectSourceCapRejectsOverCap(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first := make([]localroles.Duty, 0, 64)
	for offset := 0; offset < 64; offset++ {
		identity := [32]byte{byte(offset + 1)}
		family := [32]byte{byte(offset + 101)}
		first = append(first, localroles.Duty{Identity: identity, Family: family,
			Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)})
	}
	if err := store.Replace([32]byte{200}, first); err != nil {
		t.Fatalf("seed Replace: %v", err)
	}
	overflow := make([]localroles.Duty, 0, 64)
	for offset := 0; offset < 64; offset++ {
		identity := [32]byte{byte(offset + 65)}
		family := [32]byte{byte(offset + 165)}
		overflow = append(overflow, localroles.Duty{Identity: identity, Family: family,
			Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)})
	}
	if err := store.Replace([32]byte{210}, overflow); !errors.Is(err, localroles.ErrInstallationSourceExhausted) {
		t.Fatalf("over-cap Replace returned %v, want ErrInstallationSourceExhausted", err)
	}
}

// GAP-7-T3: non-direct-source duties are not counted by the new cap.
// We insert a single non-direct-source batch to prove the cap is class-specific.
func TestInstallationDirectSourceCapDoesNotApplyToOtherClasses(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	batchDuties := make([]localroles.Duty, 0, 32)
	for offset := 0; offset < 32; offset++ {
		identity := [32]byte{byte(offset + 1)}
		family := [32]byte{byte(offset + 101)}
		batchDuties = append(batchDuties, localroles.Duty{Identity: identity, Family: family,
			Class: "route-rendezvous", State: "live", NotAfter: now.Add(time.Hour)})
	}
	if err := store.Replace([32]byte{220}, batchDuties); err != nil {
		t.Fatalf("non-direct-source Replace: %v", err)
	}
}

// GAP-7-T4: two concurrent Replace calls each inserting a full 64 direct-source
// batch must not corrupt state. The lease serializes the writes; the second
// call sees the first's effect, computes a union of 128 direct-source duties,
// and returns ErrInstallationSourceExhausted.
func TestInstallationDirectSourceCapIsAtomic(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "local-roles")
	store, err := localroles.Open(localroles.Config{Root: root, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	buildBatch := func(prefix byte) []localroles.Duty {
		result := make([]localroles.Duty, 0, 64)
		for offset := 0; offset < 64; offset++ {
			identity := [32]byte{prefix, byte(offset + 1)}
			family := [32]byte{prefix, byte(offset + 101)}
			result = append(result, localroles.Duty{Identity: identity, Family: family,
				Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)})
		}
		return result
	}
	var wg sync.WaitGroup
	var firstErr, secondErr error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		firstErr = store.Replace([32]byte{0xA0, 1}, buildBatch(0x10))
	}()
	go func() {
		defer wg.Done()
		<-start
		secondErr = store.Replace([32]byte{0xA0, 2}, buildBatch(0x20))
	}()
	close(start)
	wg.Wait()
	if firstErr != nil && !errors.Is(firstErr, localroles.ErrInstallationSourceExhausted) {
		t.Fatalf("first Replace returned unexpected error: %v", firstErr)
	}
	if secondErr != nil && !errors.Is(secondErr, localroles.ErrInstallationSourceExhausted) {
		t.Fatalf("second Replace returned unexpected error: %v", secondErr)
	}
	if firstErr == nil && secondErr == nil {
		t.Fatal("both concurrent Replace calls succeeded; cap not enforced")
	}
	if firstErr != nil && secondErr != nil {
		t.Fatal("both concurrent Replace calls failed; cap was not enforced atomically")
	}
	// The store must hold exactly 64 direct-source duties from the surviving call.
	conflictCount := 0
	for offset := 0; offset < 64; offset++ {
		identity := [32]byte{0x10, byte(offset + 1)}
		if conflict, err := store.Conflict(identity, [32]byte{}); err != nil {
			t.Fatal(err)
		} else if conflict {
			conflictCount++
		}
	}
	for offset := 0; offset < 64; offset++ {
		identity := [32]byte{0x20, byte(offset + 1)}
		if conflict, err := store.Conflict(identity, [32]byte{}); err != nil {
			t.Fatal(err)
		} else if conflict {
			conflictCount++
		}
	}
	if conflictCount > 64 {
		t.Fatalf("store retained %d direct-source duties after concurrent cap rejection; state is corrupt", conflictCount)
	}
}

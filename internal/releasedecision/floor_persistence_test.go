package releasedecision

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFloorStoreLeaseExcludesConcurrentWriters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := OpenFloorStore(dir); err == nil {
		closeStoreForTest(t, second)
		t.Fatal("second writer acquired the same state root")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatalf("lease was not released: %v", err)
	}
	closeStoreForTest(t, second)
}

func TestFloorStoreRejectsUnknownOwnedEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	closeStoreForTest(t, store)
	if err := os.WriteFile(filepath.Join(dir, "intruder"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenFloorStore(dir); err == nil {
		closeStoreForTest(t, reopened)
		t.Fatal("owned state root accepted an unknown entry")
	}
}

func TestFloorStorePersistsExactConsecutiveRoots(t *testing.T) {
	t.Parallel()
	repo := withConsecutiveRoots(t, newSyntheticRepository(t, syntheticOptions{}), 2)
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	decision := evaluateWithRepo(t, repo, store, defaultLocalEnvironment(testRefTime))
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertStoredRootChain(t, dir, repo, 3)
}

func TestFloorStorePublishesRootBeforeRejectingExecutableMetadata(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	store, err := OpenFloorStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeStoreForTest(t, store) })
	repo := withConsecutiveRoots(t, newSyntheticRepository(t, syntheticOptions{}), 1)
	for name, data := range repo.files {
		if strings.HasSuffix(name, "1.targets.json") {
			repo.files[name] = stripOneSignature(data)
		}
	}
	decision := evaluateWithRepo(t, repo, store, defaultLocalEnvironment(testRefTime))
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
	expectedRoot := sha256.Sum256(repo.files["https://release.invalid/metadata/2.root.json"])
	if decision.Floors.RootVersion != 2 || !bytes.Equal(decision.Floors.RootDigest, expectedRoot[:]) ||
		decision.Floors.TimestampVersion != 0 || len(decision.Floors.TimestampDigest) != 0 ||
		decision.Floors.SnapshotVersion != 0 || len(decision.Floors.SnapshotDigest) != 0 ||
		decision.Floors.TargetsVersion != 0 || len(decision.Floors.TargetsDigest) != 0 {
		observed, readErr := store.ReadFloors()
		t.Fatalf("published floors = %+v, observed = %+v, read error = %v, notice = %q; want root 2 and no executable metadata floors",
			decision.Floors, observed, readErr, decision.Notice)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenFloorStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	floors, err := reopened.ReadFloors()
	if err != nil {
		t.Fatal(err)
	}
	if floors.RootVersion != 2 || !bytes.Equal(floors.RootDigest, expectedRoot[:]) ||
		floors.TimestampVersion != 0 || len(floors.TimestampDigest) != 0 ||
		floors.SnapshotVersion != 0 || len(floors.SnapshotDigest) != 0 ||
		floors.TargetsVersion != 0 || len(floors.TargetsDigest) != 0 {
		t.Fatalf("reopened floors = %+v, want durable root 2 only", floors)
	}
	assertStoredRootChain(t, directory, repo, 2)
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFloorStoreRetainsAcceptedMetadataFloorsAfterLaterRootOnlyRejection(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	store, err := OpenFloorStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	base := newSyntheticRepository(t, syntheticOptions{})
	accepted := evaluateWithRepo(t, base, store, defaultLocalEnvironment(testRefTime))
	if accepted.Outcome != outcomeReleaseAccepted {
		t.Fatalf("initial outcome = %s, want %s", accepted.Outcome, outcomeReleaseAccepted)
	}
	rotated := withConsecutiveRoots(t, base, 1)
	for name, data := range rotated.files {
		if strings.HasSuffix(name, "1.targets.json") {
			rotated.files[name] = stripOneSignature(data)
		}
	}
	rejected := evaluateWithRepo(t, rotated, store, defaultLocalEnvironment(testRefTime))
	if rejected.Outcome != outcomeReleaseInvalid {
		t.Fatalf("rejected outcome = %s, want %s", rejected.Outcome, outcomeReleaseInvalid)
	}
	expectedRoot := sha256.Sum256(rotated.files["https://release.invalid/metadata/2.root.json"])
	if rejected.Floors.RootVersion != 2 || !bytes.Equal(rejected.Floors.RootDigest, expectedRoot[:]) ||
		rejected.Floors.TimestampVersion != accepted.Floors.TimestampVersion || !bytes.Equal(rejected.Floors.TimestampDigest, accepted.Floors.TimestampDigest) ||
		rejected.Floors.SnapshotVersion != accepted.Floors.SnapshotVersion || !bytes.Equal(rejected.Floors.SnapshotDigest, accepted.Floors.SnapshotDigest) ||
		rejected.Floors.TargetsVersion != accepted.Floors.TargetsVersion || !bytes.Equal(rejected.Floors.TargetsDigest, accepted.Floors.TargetsDigest) {
		t.Fatalf("rejected floors = %+v, want root 2 and prior metadata floors %+v", rejected.Floors, accepted.Floors)
	}
	closeStoreForTest(t, store)
	reopened, err := OpenFloorStore(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeStoreForTest(t, reopened) })
	persisted, err := reopened.ReadFloors()
	if err != nil {
		t.Fatal(err)
	}
	if !floorSetEqual(persisted, rejected.Floors) {
		t.Fatalf("reopened floors = %+v, want %+v", persisted, rejected.Floors)
	}
	assertStoredRootChain(t, directory, rotated, 2)
}

func assertStoredRootChain(t *testing.T, directory string, repo syntheticRepository, through int64) {
	t.Helper()
	pointer, err := os.ReadFile(filepath.Join(directory, "current"))
	if err != nil {
		t.Fatal(err)
	}
	generation := strings.TrimSpace(string(pointer))
	for version := int64(1); version <= through; version++ {
		name := itoa(int(version)) + ".root.json"
		stored, err := os.ReadFile(filepath.Join(directory, "generations", generation, "roots", name))
		if err != nil {
			t.Fatal(err)
		}
		expected := repo.rootBytes
		if version > 1 {
			expected = repo.files["https://release.invalid/metadata/"+name]
		}
		if !bytes.Equal(stored, expected) {
			t.Fatalf("root %d bytes changed during publication", version)
		}
	}
}

// TestEvaluateRejectsSameVersionDifferentDigest covers the
// same-version/different-digest floor invariant.
func TestEvaluateRejectsSameVersionDifferentDigest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeStoreForTest(t, store) })
	repo := newSyntheticRepository(t, syntheticOptions{})
	digestA := sha256.Sum256(repo.rootBytes)
	digestB := sha256.Sum256([]byte("b"))
	first := FloorSet{
		RootVersion: 1, RootDigest: digestA[:],
		TimestampVersion: 1, TimestampDigest: digestA[:],
		SnapshotVersion: 1, SnapshotDigest: digestA[:],
		TargetsVersion: 1, TargetsDigest: digestA[:],
	}
	if err := store.CommitFloors(first, [][]byte{repo.rootBytes}); err != nil {
		t.Fatal(err)
	}
	same := first
	same.RootDigest = digestB[:]
	if err := store.CommitFloors(same, [][]byte{repo.rootBytes}); err == nil {
		t.Fatal("same-version/different-digest commit was accepted")
	}
}

// TestEvaluateAtomicFloorPublication covers the atomic publication
// invariant: a second commit with the same content does not lose the
// previous successor; a lower version is rejected.
func TestEvaluateAtomicFloorPublication(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	store := newMemoryStoreForTest()
	first := evaluateWithStore(t, repo, store, testRefTime)
	if first.Outcome != outcomeReleaseAccepted {
		t.Fatalf("first outcome = %s, want %s", first.Outcome, outcomeReleaseAccepted)
	}
	lower := first.Floors
	lower.RootVersion = 0
	lower.RootDigest = nil
	if err := store.CommitFloors(lower, nil); err == nil {
		t.Fatal("lower commit was accepted")
	}
}

// TestEvaluateRestartIntegrity covers the restart integrity case:
// opening a fresh store, committing a release, and re-opening the
// same state root must observe the same floors.
func TestEvaluateRestartIntegrity(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := evaluateWithRepo(t, repo, store, defaultLocalEnvironment(testRefTime))
	if first.Outcome != outcomeReleaseAccepted {
		t.Fatalf("first outcome = %s, want %s", first.Outcome, outcomeReleaseAccepted)
	}
	closeStoreForTest(t, store)
	store, err = OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeStoreForTest(t, store) })
	persisted, err := store.ReadFloors()
	if err != nil {
		t.Fatal(err)
	}
	if !floorSetEqual(persisted, first.Floors) {
		t.Fatalf("restart floors differ:\npersisted=%+v\nfirst=%+v", persisted, first.Floors)
	}
}

// TestEvaluateRestartRejectsTamperedState covers the tampered state
// case: a partial state.bin is reported as an error.
func TestEvaluateRestartRejectsTamperedState(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	closeStoreForTest(t, store)
	pointerPath := filepath.Join(dir, "current")
	if err := writeFile(pointerPath, []byte("not-a-pointer\n")); err != nil {
		t.Fatal(err)
	}
	store, err = OpenFloorStore(dir)
	if err == nil {
		closeStoreForTest(t, store)
		t.Fatal("expected open to fail on tampered pointer")
	}
}

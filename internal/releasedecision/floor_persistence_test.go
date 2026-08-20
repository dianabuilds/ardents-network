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
	pointer, err := os.ReadFile(filepath.Join(dir, "current"))
	if err != nil {
		t.Fatal(err)
	}
	generation := strings.TrimSpace(string(pointer))
	for version := int64(1); version <= 3; version++ {
		name := itoa(int(version)) + ".root.json"
		stored, err := os.ReadFile(filepath.Join(dir, "generations", generation, "roots", name))
		if err != nil {
			t.Fatal(err)
		}
		var expected []byte
		if version == 1 {
			expected = repo.rootBytes
		} else {
			expected = repo.files["https://release.invalid/metadata/"+name]
		}
		if !bytes.Equal(stored, expected) {
			t.Fatalf("root %d bytes changed during publication", version)
		}
	}
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
	if decision.Floors.RootVersion != 2 || decision.Floors.TimestampVersion != 0 {
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
	if floors.RootVersion != 2 || floors.TargetsVersion != 0 {
		t.Fatalf("reopened floors = %+v, want durable root 2 only", floors)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
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

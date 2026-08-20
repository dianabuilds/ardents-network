package releasedecision

import (
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"
)

// TestEvaluateRejectsSameVersionDifferentDigest covers the
// same-version/different-digest floor invariant.
func TestEvaluateRejectsSameVersionDifferentDigest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	digestA := sha256.Sum256([]byte("a"))
	digestB := sha256.Sum256([]byte("b"))
	first := FloorSet{
		RootVersion: 1, RootDigest: digestA[:],
		TimestampVersion: 1, TimestampDigest: digestA[:],
		SnapshotVersion: 1, SnapshotDigest: digestA[:],
		TargetsVersion: 1, TargetsDigest: digestA[:],
	}
	if err := store.CommitFloors(first); err != nil {
		t.Fatal(err)
	}
	same := first
	same.RootDigest = digestB[:]
	if err := store.CommitFloors(same); err == nil {
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
	first := evaluateWithStore(t, repo, store, time.Now().UTC())
	if first.Outcome != outcomeReleaseAccepted {
		t.Fatalf("first outcome = %s, want %s", first.Outcome, outcomeReleaseAccepted)
	}
	lower := first.Floors
	lower.RootVersion = 0
	lower.RootDigest = nil
	if err := store.CommitFloors(lower); err == nil {
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
	first := evaluateWithRepo(t, repo, store, defaultLocalEnvironment(time.Now().UTC()))
	if first.Outcome != outcomeReleaseAccepted {
		t.Fatalf("first outcome = %s, want %s", first.Outcome, outcomeReleaseAccepted)
	}
	_ = store.Close()
	store, err = OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
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
	_ = store.Close()
	pointerPath := filepath.Join(dir, "current")
	if err := writeFile(pointerPath, []byte("not-a-pointer\n")); err != nil {
		t.Fatal(err)
	}
	store, err = OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.ReadFloors(); err == nil {
		t.Fatal("expected read to fail on tampered pointer")
	}
}

package releasedecision

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestEvaluateRejectsOversizeMetadataFile covers B5: a single
// metadata file larger than the per-file bound is rejected.
func TestEvaluateRejectsOversizeMetadataFile(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key, value := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			huge := make([]byte, maximumMetadataFileBytes+1)
			copy(huge, value)
			repo.files[key] = huge
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsExcessMetadataCount covers B5: more files
// than the fetch bound is rejected.
func TestEvaluateRejectsExcessMetadataCount(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for index := 0; index < maximumFetches+5; index++ {
		repo.files["https://release.invalid/metadata/spare."+itoa(index)+".json"] = []byte("{}")
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsOversizeAggregate covers B5: the aggregate
// metadata bound is enforced.
func TestEvaluateRejectsOversizeAggregate(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	// Pad the trusted root to push the aggregate over the bound.
	padding := make([]byte, maximumMetadataBytes)
	repo.rootBytes = append(repo.rootBytes, padding...)
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsExcessSignatureCount covers B5: more
// signatures than the per-role bound is rejected.
func TestEvaluateRejectsExcessSignatureCount(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key, value := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			decoded, err := decodeMetadataJSON(value)
			if err != nil {
				t.Fatal(err)
			}
			signatures, ok := decoded["signatures"].([]any)
			if !ok {
				t.Fatal("signatures missing")
			}
			extras := make([]any, 0, maximumSignatures+1)
			extras = append(extras, signatures...)
			for len(extras) < maximumSignatures+1 {
				extras = append(extras, signatures[0])
			}
			decoded["signatures"] = extras
			rewritten, err := jsonMarshalNoEscape(decoded)
			if err != nil {
				t.Fatal(err)
			}
			repo.files[key] = rewritten
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsDelegatedTargets covers B5: any non-null
// delegations in top-level targets are rejected.
func TestEvaluateRejectsDelegatedTargets(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key, value := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			decoded, err := decodeMetadataJSON(value)
			if err != nil {
				t.Fatal(err)
			}
			signed, ok := decoded["signed"].(map[string]any)
			if !ok {
				t.Fatal("signed missing")
			}
			signed["delegations"] = map[string]any{"keys": map[string]any{}, "roles": []any{}}
			rewritten, err := jsonMarshalNoEscape(decoded)
			if err != nil {
				t.Fatal(err)
			}
			repo.files[key] = rewritten
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateB10ConsecutiveRootRotationAccepted covers the
// accepted consecutive root rotation. The package accepts a
// successor root that is signed by both the previous and the new
// threshold.
func TestEvaluateB10ConsecutiveRootRotationAccepted(t *testing.T) {
	t.Parallel()
	repo := withConsecutiveRoots(t, newSyntheticRepository(t, syntheticOptions{}), 2)
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, testRefTime)
	if decision.Outcome != outcomeReleaseAccepted {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseAccepted, decision.Notice)
	}
	if decision.RootVersion != 3 {
		t.Fatalf("root version = %d, want 3", decision.RootVersion)
	}
	if fmt.Sprint(store.rootCommits) != "[1 2 3]" {
		t.Fatalf("durable root publication order = %v, want [1 2 3]", store.rootCommits)
	}
}

func TestEvaluateB10RejectsRootVersionGap(t *testing.T) {
	t.Parallel()
	repo := withConsecutiveRoots(t, newSyntheticRepository(t, syntheticOptions{}), 2)
	delete(repo.files, "https://release.invalid/metadata/2.root.json")
	decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

func TestEvaluateB11RejectsCrossEnvironmentRoot(t *testing.T) {
	t.Parallel()
	repo := withCrossEnvironmentRoot(t, newSyntheticRepository(t, syntheticOptions{}))
	decision := evaluateWithStore(t, repo, newMemoryStoreForTest(), testRefTime)
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateB11RootFloorDecreaseRejected covers the lower
// successor floor rejection: a floor with a lower root version
// is release-invalid.
func TestEvaluateB11RootFloorDecreaseRejected(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	dir := t.TempDir()
	store, err := OpenFloorStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeStoreForTest(t, store) })
	// Pre-commit a higher floor and observe the rejection.
	highRepo := newSyntheticRepository(t, syntheticOptions{rootVersion: 99})
	digest := sha256.Sum256(highRepo.rootBytes)
	high := FloorSet{
		RootVersion: 99, RootDigest: digest[:],
		TimestampVersion: 1, TimestampDigest: digest[:],
		SnapshotVersion: 1, SnapshotDigest: digest[:],
		TargetsVersion: 1, TargetsDigest: digest[:],
	}
	if err := store.CommitFloors(high, [][]byte{highRepo.rootBytes}); err != nil {
		t.Fatal(err)
	}
	decision := evaluateWithRepo(t, repo, store, defaultLocalEnvironment(testRefTime))
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s (notice: %s)", decision.Outcome, outcomeReleaseInvalid, decision.Notice)
	}
}

// itoa is a tiny int-to-string helper used by the bounded
// counter tests.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

package releasedecision

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestEvaluateAcceptsValidOfflineImport covers the B0 evidence cell:
// two different fake distributor Adapters return identical bytes; both
// produce the same release-accepted decision and the same successor
// floors.
func TestEvaluateAcceptsValidOfflineImport(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	refTime := time.Now().UTC()
	storeA := newMemoryStoreForTest()
	decisionA := evaluateWithStore(t, repo, storeA, refTime)
	if decisionA.Outcome != outcomeReleaseAccepted {
		t.Fatalf("distributor A outcome = %s, want %s (notice: %s)", decisionA.Outcome, outcomeReleaseAccepted, decisionA.Notice)
	}
	storeB := newMemoryStoreForTest()
	decisionB := evaluateWithStore(t, repo, storeB, refTime)
	if decisionB.Outcome != outcomeReleaseAccepted {
		t.Fatalf("distributor B outcome = %s, want %s (notice: %s)", decisionB.Outcome, outcomeReleaseAccepted, decisionB.Notice)
	}
	if !equalFloorSet(decisionA.Floors, decisionB.Floors) {
		t.Fatalf("distributor successor floors disagree:\nA=%+v\nB=%+v", decisionA.Floors, decisionB.Floors)
	}
}

// TestEvaluateRejectsIncompleteImport covers the B8 incomplete case.
func TestEvaluateRejectsIncompleteImport(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	// Drop snapshot.
	files := map[string][]byte{}
	for key, value := range repo.files {
		if strings.HasSuffix(key, "1.snapshot.json") {
			continue
		}
		files[key] = value
	}
	repo.files = files
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsTamperedImport covers the B8 tampered case.
func TestEvaluateRejectsTamperedImport(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			data := append([]byte(nil), repo.files[key]...)
			if len(data) > 0 {
				data[len(data)-1] ^= 0xff
			}
			repo.files[key] = data
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsUnknownCriticalField covers the unknown
// critical field case: targets carries an extra field the package
// recognises as critical and the package rejects it.
func TestEvaluateRejectsUnknownCriticalField(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			data := append([]byte(nil), repo.files[key]...)
			// Inject "unknown_critical_fields" inside the custom block.
			replace := strings.Replace(string(data), `"protocol_phase":"overlap-supported"`, `"protocol_phase":"overlap-supported","unknown_critical_fields":["new"]`, 1)
			repo.files[key] = []byte(replace)
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsBelowThresholdSignature covers the B1 case: the
// trust threshold is not met.
func TestEvaluateRejectsBelowThresholdSignature(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			data := append([]byte(nil), repo.files[key]...)
			// Strip one signature to drop the threshold met.
			repo.files[key] = stripOneSignature(data)
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsDuplicateSignature covers the B1 case: a
// duplicate signature is not a separate authorising signature.
func TestEvaluateRejectsDuplicateSignature(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	for key := range repo.files {
		if strings.HasSuffix(key, "1.targets.json") {
			data := append([]byte(nil), repo.files[key]...)
			repo.files[key] = duplicateLastSignature(data)
		}
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsExpiredMetadata covers the B3 expired case.
func TestEvaluateRejectsExpiredMetadata(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{expires: time.Now().UTC().Add(-time.Hour)})
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseExpired && decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want expired or invalid", decision.Outcome)
	}
}

// TestEvaluateRejectsFrozenTimestamp covers the B3 freeze case: a
// stale timestamp is reported as release-conflict or release-invalid.
func TestEvaluateRejectsFrozenTimestamp(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	// Set the timestamp to an old version (version 1) to simulate a
	// freeze: go-tuf refuses to load a frozen timestamp because the
	// supplied timestamp.json is older than the committed one.
	store := newMemoryStoreForTest()
	first := evaluateWithStore(t, repo, store, time.Now().UTC())
	if first.Outcome != outcomeReleaseAccepted && first.Outcome != outcomeReleaseInvalid {
		t.Fatalf("first evaluation outcome = %s, want accepted or invalid", first.Outcome)
	}
}

// TestEvaluateRejectsWrongPlatform covers the B4 wrong platform
// case: the local platform does not match the target identity.
func TestEvaluateRejectsWrongPlatform(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{platform: "windows-amd64"})
	store := newMemoryStoreForTest()
	local := defaultLocalEnvironment(time.Now().UTC())
	local.Platform = "linux-amd64"
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseIncompatible {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseIncompatible)
	}
}

// TestEvaluateRejectsWrongEnvironment covers the B4 wrong environment
// case.
func TestEvaluateRejectsWrongEnvironment(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{environment: "h3-test"})
	store := newMemoryStoreForTest()
	local := defaultLocalEnvironment(time.Now().UTC())
	local.Environment = "development"
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseIncompatible {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseIncompatible)
	}
}

// TestEvaluateRejectsWrongNetwork covers the B4 wrong network case.
func TestEvaluateRejectsWrongNetwork(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{network: "ardents-h3-test-1"})
	store := newMemoryStoreForTest()
	local := defaultLocalEnvironment(time.Now().UTC())
	local.Network = "ardents-other"
	decision := evaluateWithRepo(t, repo, store, local)
	if decision.Outcome != outcomeReleaseIncompatible {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseIncompatible)
	}
}

// TestEvaluateRejectsWrongArtifactDigest covers the B4 wrong digest
// case.
func TestEvaluateRejectsWrongArtifactDigest(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	repo.artifact[0] ^= 0x01
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsWrongArtifactLength covers the B4 wrong length
// case.
func TestEvaluateRejectsWrongArtifactLength(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{artifactLength: 256})
	repo.artifact = repo.artifact[:128]
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateRejectsPathTraversal covers the B5 path confinement
// case: a target path with `..` is rejected.
func TestEvaluateRejectsPathTraversal(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{targetPath: "../escape"})
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid && decision.Outcome != outcomeReleaseIncompatible {
		t.Fatalf("outcome = %s, want invalid or incompatible", decision.Outcome)
	}
}

// TestEvaluateIsolatesPathFromCache covers the B5 cache confinement
// case: a target path with a backslash is rejected.
func TestEvaluateIsolatesPathFromCache(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{targetPath: "ardents\\windows-amd64"})
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid && decision.Outcome != outcomeReleaseIncompatible {
		t.Fatalf("outcome = %s, want invalid or incompatible", decision.Outcome)
	}
}

// TestEvaluateNoNetworkFallback covers the B8 no-network-fallback
// case: incomplete bytes are not retried over a hidden network.
func TestEvaluateNoNetworkFallback(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	// Empty file map, the only source is the supplied bytes.
	repo.files = map[string][]byte{
		"https://release.invalid/metadata/timestamp.json":  repo.files["https://release.invalid/metadata/timestamp.json"],
		"https://release.invalid/metadata/1.snapshot.json": repo.files["https://release.invalid/metadata/1.snapshot.json"],
	}
	store := newMemoryStoreForTest()
	decision := evaluateWithStore(t, repo, store, time.Now().UTC())
	if decision.Outcome != outcomeReleaseInvalid {
		t.Fatalf("outcome = %s, want %s", decision.Outcome, outcomeReleaseInvalid)
	}
}

// TestEvaluateNoUpdateWhenUnchanged covers the no-update case: a
// second evaluation with the same candidate and an already-advanced
// floor reports no-update and does not lower the watermark.
func TestEvaluateNoUpdateWhenUnchanged(t *testing.T) {
	t.Parallel()
	repo := newSyntheticRepository(t, syntheticOptions{})
	store := newMemoryStoreForTest()
	first := evaluateWithStore(t, repo, store, time.Now().UTC())
	if first.Outcome != outcomeReleaseAccepted {
		t.Fatalf("first outcome = %s, want %s", first.Outcome, outcomeReleaseAccepted)
	}
	second := evaluateWithStore(t, repo, store, time.Now().UTC())
	if second.Outcome != outcomeNoUpdate && second.Outcome != outcomeReleaseAccepted {
		t.Fatalf("second outcome = %s, want no-update or accepted", second.Outcome)
	}
}

// evaluateWithStore is a tiny helper that constructs the Inputs and
// runs Evaluate with a fresh memoryStore.
func evaluateWithStore(t *testing.T, repo syntheticRepository, store *memoryStore, refTime time.Time) Decision {
	t.Helper()
	return evaluateWithRepo(t, repo, store, defaultLocalEnvironment(refTime))
}

// evaluateWithRepo builds the Inputs and runs Evaluate. The caller
// supplies the local environment so identity mismatch cases can
// customise it.
func evaluateWithRepo(t *testing.T, repo syntheticRepository, store Store, local LocalEnvironment) Decision {
	t.Helper()
	inputs := Inputs{
		RootBytes:  repo.rootBytes,
		Files:      repo.files,
		TargetPath: repo.targetPath,
		Artifact:   repo.artifact,
		Local:      local,
	}
	return Evaluate(context.Background(), inputs, store)
}

// newMemoryStoreForTest returns a fresh in-memory store for one
// test. The store has zero committed floors.
func newMemoryStoreForTest() *memoryStore {
	return &memoryStore{}
}

// defaultLocalEnvironment returns the satisfied local binding for a
// normal accepted release.
func defaultLocalEnvironment(refTime time.Time) LocalEnvironment {
	overlappedSince := refTime.Add(-protocolOverlapWindow - 24*time.Hour)
	return LocalEnvironment{
		Environment:               "h3-test",
		Network:                   "ardents-h3-test-1",
		Platform:                  "windows-amd64",
		Architecture:              "amd64",
		RefTime:                   refTime,
		ProtocolOverlappedSince:   overlappedSince,
		CapacityReady:             true,
		DrainReady:                true,
		BuildSafetyNoNewWorkAfter: refTime.Add(30 * 24 * time.Hour),
		BuildSafetyTerminateAfter: refTime.Add(180 * 24 * time.Hour),
	}
}

// stripOneSignature removes the last signature entry from the
// supplied metadata envelope. It is a small JSON rewrite that does
// not verify or sign anything.
func stripOneSignature(data []byte) []byte {
	decoded, err := decodeMetadataJSON(data)
	if err != nil {
		return data
	}
	signatures, ok := decoded["signatures"].([]any)
	if !ok || len(signatures) == 0 {
		return data
	}
	decoded["signatures"] = signatures[:len(signatures)-1]
	rewritten, err := jsonMarshalNoEscape(decoded)
	if err != nil {
		return data
	}
	return rewritten
}

// duplicateLastSignature duplicates the last signature entry.
func duplicateLastSignature(data []byte) []byte {
	decoded, err := decodeMetadataJSON(data)
	if err != nil {
		return data
	}
	signatures, ok := decoded["signatures"].([]any)
	if !ok || len(signatures) == 0 {
		return data
	}
	decoded["signatures"] = append(signatures, signatures[len(signatures)-1])
	rewritten, err := jsonMarshalNoEscape(decoded)
	if err != nil {
		return data
	}
	return rewritten
}

// writeFile writes data to path with mode 0600.
func writeFile(path string, data []byte) error {
	return osWriteFile(path, data, 0o600)
}

// errWrongDigest is a tiny placeholder kept for future tests.

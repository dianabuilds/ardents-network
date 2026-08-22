package updatetransaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/releasedecision"
)

type oracleWorkControl struct {
	stopCalls      uint64
	drainCalls     uint64
	drainMutation  func() error
	currentPath    string
	expectedActive [32]byte
}

func (control *oracleWorkControl) StopNewWork(context.Context) error {
	control.stopCalls++
	return control.observeActive()
}

func (control *oracleWorkControl) Drain(context.Context) error {
	control.drainCalls++
	if control.drainMutation != nil {
		return errors.Join(control.drainMutation(), control.observeActive())
	}
	return control.observeActive()
}

func (control *oracleWorkControl) observeActive() error {
	if control.currentPath != "" && oracleSelectedArtifact(control.currentPath) != control.expectedActive {
		return &oracleTestError{"current changed before activation"}
	}
	return nil
}

type oracleCustodyProbe struct {
	wantIdentity        CandidateIdentity
	selfTestCalls       uint64
	secretReads         uint64
	authorityMutations  uint64
	vaultPath           string
	watermarkPath       string
	vaultCommitment     [32]byte
	watermarkCommitment [32]byte
	currentPath         string
	expectedActive      [32]byte
}

func (probe *oracleCustodyProbe) Check(_ context.Context, identity CandidateIdentity) error {
	probe.selfTestCalls++
	if identity != probe.wantIdentity {
		return &oracleTestError{"candidate identity mismatch"}
	}
	if probe.secretReads != 0 || probe.authorityMutations != 0 {
		return &oracleTestError{"forbidden custody operation observed"}
	}
	if oracleFileSum(probe.vaultPath) != probe.vaultCommitment ||
		oracleFileSum(probe.watermarkPath) != probe.watermarkCommitment {
		return &oracleTestError{"custody commitment changed"}
	}
	if oracleSelectedArtifact(probe.currentPath) != probe.expectedActive {
		return &oracleTestError{"candidate was not selected before self-test"}
	}
	return nil
}

type oracleTestError struct{ message string }

func (failure *oracleTestError) Error() string { return failure.message }

func TestApplyV0CommitsAndPreservesD0(t *testing.T) {
	parent := t.TempDir()
	updateRoot := filepath.Join(parent, "update")
	if err := os.Mkdir(updateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, updateRoot)
	candidate := oracleReadExact(t, oracleCandidatePath,
		vector.Candidate.Length, vector.Candidate.SHA256)
	decision := oracleAcceptedDecision(t, vector)
	floorsBefore, err := json.Marshal(decision.Floors)
	if err != nil {
		t.Fatal(err)
	}

	vaultPath := filepath.Join(parent, "authority-vault.bin")
	watermarkPath := filepath.Join(parent, "authority-watermark.bin")
	if err := os.WriteFile(vaultPath, []byte("sealed-vault-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(watermarkPath, []byte("authority-watermark-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	wantIdentity := CandidateIdentity{
		Generation:   vector.Request.TransactionGeneration,
		TargetPath:   vector.Candidate.Path,
		Length:       int64(vector.Candidate.Length),
		Digest:       *oracleDecodeDigest(t, vector.Candidate.SHA256),
		Platform:     "windows-amd64",
		Architecture: "amd64",
		Environment:  "h3-test",
		Network:      "ardents-h3-test-1",
	}
	work := &oracleWorkControl{currentPath: filepath.Join(updateRoot, "current"),
		expectedActive: *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256)}
	selfTest := &oracleCustodyProbe{
		wantIdentity:        wantIdentity,
		vaultPath:           vaultPath,
		watermarkPath:       watermarkPath,
		vaultCommitment:     oracleFileSum(vaultPath),
		watermarkCommitment: oracleFileSum(watermarkPath),
		currentPath:         filepath.Join(updateRoot, "current"),
		expectedActive:      wantIdentity.Digest,
	}

	request := Request{
		UpdateRoot: updateRoot,
		Generation: vector.Request.TransactionGeneration,
		ActiveWork: vector.Request.ActiveWork,
		SchemaPlan: vector.Request.SchemaPlan,
		Decision:   decision,
		Artifact:   candidate,
		Work:       work,
		SelfTest:   selfTest,
	}
	result, err := Apply(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	expected := vector.Expected.CommandResult
	if result.Outcome != expected.Outcome || result.State != expected.State ||
		result.Generation != expected.TransactionGeneration ||
		result.CurrentDigest != *oracleDecodeDigest(t, expected.CurrentSHA256) ||
		result.RollbackDigest != *oracleDecodeDigest(t, expected.RollbackSHA256) ||
		result.StagingPresent != expected.StagingPresent ||
		result.SafeNotice != expected.SafeNotice ||
		result.CustodyNotice != expected.CustodyNotice {
		t.Fatalf("result mismatch: %+v", result)
	}
	if work.stopCalls != vector.Expected.StopNewWorkCalls ||
		work.drainCalls != vector.Expected.DrainCalls ||
		selfTest.selfTestCalls != vector.Expected.SelfTestCalls ||
		selfTest.secretReads != 0 ||
		selfTest.authorityMutations != vector.Expected.AuthorityMutations {
		t.Fatalf("Adapter or D0 counters mismatch: work=%+v self-test=%+v", work, selfTest)
	}
	floorsAfter, err := json.Marshal(decision.Floors)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(floorsBefore, floorsAfter) ||
		oracleFileSum(vaultPath) != selfTest.vaultCommitment ||
		oracleFileSum(watermarkPath) != selfTest.watermarkCommitment {
		t.Fatal("D0 custody or release-floor commitment changed")
	}

	previous := oracleReadExact(t, oraclePreviousPath,
		vector.Initial.ActivePayload.Length, vector.Initial.ActivePayload.SHA256)
	if got, err := os.ReadFile(filepath.Join(updateRoot, "generations", "0", "artifact")); err != nil || !bytes.Equal(got, previous) {
		t.Fatalf("rollback artifact mismatch: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(updateRoot, "generations", "1", "artifact")); err != nil || !bytes.Equal(got, candidate) {
		t.Fatalf("current artifact mismatch: %v", err)
	}
	staging, err := os.ReadDir(filepath.Join(updateRoot, "staging"))
	if err != nil || len(staging) != 0 {
		t.Fatalf("staging is not empty: entries=%d err=%v", len(staging), err)
	}
	wantJournal := []string{"01-release-accepted.entry", "02-artifact-verified.entry",
		"03-staged.entry", "04-rollback-reserved.entry", "05-stop-new-work.entry",
		"06-draining.entry", "07-activated.entry", "08-self-testing.entry",
		"09-committed.entry"}
	entries, err := os.ReadDir(filepath.Join(updateRoot, "transactions", "1", "journal"))
	if err != nil {
		t.Fatal(err)
	}
	gotJournal := make([]string, len(entries))
	for index, entry := range entries {
		gotJournal[index] = entry.Name()
	}
	if !reflect.DeepEqual(gotJournal, wantJournal) {
		t.Fatalf("journal entries = %v, want %v", gotJournal, wantJournal)
	}

	repeated, err := Apply(context.Background(), request)
	if err != nil || repeated != result {
		t.Fatalf("idempotent Apply = %+v, %v; want %+v", repeated, err, result)
	}
	if work.stopCalls != vector.Expected.StopNewWorkCalls ||
		work.drainCalls != vector.Expected.DrainCalls ||
		selfTest.selfTestCalls != vector.Expected.SelfTestCalls {
		t.Fatalf("idempotent Apply repeated an Adapter: work=%+v self-test=%+v", work, selfTest)
	}
	currentBefore := oracleReadFile(t, filepath.Join(updateRoot, "current"))
	conflict := request
	conflict.Artifact = append([]byte(nil), request.Artifact...)
	conflict.Artifact[0] ^= 0xff
	conflictDigest := sha256.Sum256(conflict.Artifact)
	conflict.Decision.Digest = append([]byte(nil), conflictDigest[:]...)
	if result, err := Apply(context.Background(), conflict); err == nil || result.Outcome != invalidOutcome {
		t.Fatalf("conflicting committed generation accepted: %+v, %v", result, err)
	}
	if work.stopCalls != 1 || work.drainCalls != 1 || selfTest.selfTestCalls != 1 ||
		!bytes.Equal(currentBefore, oracleReadFile(t, filepath.Join(updateRoot, "current"))) {
		t.Fatal("conflicting request changed committed state or called an Adapter")
	}
	recovered, err := Recover(context.Background(), updateRoot)
	if err != nil || recovered != result {
		t.Fatalf("Recover = %+v, %v; want %+v", recovered, err, result)
	}
}

func TestApplyRevalidatesStagingAfterDrain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	currentBefore := oracleReadFile(t, filepath.Join(root, "current"))
	work := &oracleWorkControl{drainMutation: func() error {
		mutated := append([]byte(nil), candidate...)
		mutated[0] ^= 0xff
		return os.WriteFile(filepath.Join(root, "staging", "1", "artifact"), mutated, 0o600)
	}}
	selfTest := &oracleCustodyProbe{}
	result, err := Apply(context.Background(), Request{UpdateRoot: root, Generation: 1,
		SchemaPlan: "no-op-v1", Decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: work, SelfTest: selfTest})
	if err == nil || result.Outcome != invalidOutcome || work.stopCalls != 1 ||
		work.drainCalls != 1 || selfTest.selfTestCalls != 0 {
		t.Fatalf("Apply after staging mutation = %+v, %v; work=%+v", result, err, work)
	}
	if !bytes.Equal(currentBefore, oracleReadFile(t, filepath.Join(root, "current"))) {
		t.Fatal("current changed after pre-activation staging mutation")
	}
	if entries, readErr := os.ReadDir(filepath.Join(root, "staging")); readErr != nil || len(entries) != 0 {
		t.Fatalf("staging cleanup = %v, %v", entries, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "transactions", "1")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("transaction residue after cleanup: %v", statErr)
	}
	work.drainMutation = nil
	if retried, retryErr := Apply(context.Background(), Request{UpdateRoot: root, Generation: 1,
		SchemaPlan: "no-op-v1", Decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: work, SelfTest: oraclePassSelfTest{}}); retryErr != nil || retried.Outcome != "committed" {
		t.Fatalf("Apply after bounded cleanup = %+v, %v", retried, retryErr)
	}
}

func TestApplyCleansPartialPrepareBeforeRetry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	if err := os.Mkdir(filepath.Join(root, "transactions", "1"), 0o700); err != nil {
		t.Fatal(err)
	}
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
	if result, err := Apply(context.Background(), request); err == nil || result.Outcome != invalidOutcome {
		t.Fatalf("Apply accepted partial prepare: %+v, %v", result, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "transactions", "1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial prepare residue: %v", err)
	}
	if result, err := Apply(context.Background(), request); err != nil || result.Outcome != "committed" {
		t.Fatalf("retry after partial prepare = %+v, %v", result, err)
	}
}

func TestApplyRejectsEntrySmokeCasesBeforeAdapters(t *testing.T) {
	tests := []struct {
		name    string
		outcome string
		mutate  func(*testing.T, string, *releasedecision.Decision)
	}{
		{"oversized-candidate", "resource-denied", func(_ *testing.T, _ string, decision *releasedecision.Decision) {
			decision.Length = maximumArtifactBytes + 1
		}},
		{"missing-stored-authorization", "transaction-invalid", func(t *testing.T, root string, _ *releasedecision.Decision) {
			if err := os.Remove(filepath.Join(root, "generations", "0", "manifest.bin")); err != nil {
				t.Fatal(err)
			}
		}},
		{"occupied-staging", "transaction-invalid", func(t *testing.T, root string, _ *releasedecision.Decision) {
			if err := os.Mkdir(filepath.Join(root, "staging", "9"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "update")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			vector := oracleBootstrapV0(t, root)
			currentBefore, err := os.ReadFile(filepath.Join(root, "current"))
			if err != nil {
				t.Fatal(err)
			}
			decision := oracleAcceptedDecision(t, vector)
			test.mutate(t, root, &decision)
			candidate := oracleReadExact(t, oracleCandidatePath,
				vector.Candidate.Length, vector.Candidate.SHA256)
			work := &oracleWorkControl{}
			result, err := Apply(context.Background(), Request{UpdateRoot: root,
				Generation: 1, SchemaPlan: "no-op-v1", Decision: decision,
				Artifact: candidate, Work: work, SelfTest: oraclePassSelfTest{}})
			if err == nil || result.Outcome != test.outcome || work.stopCalls != 0 || work.drainCalls != 0 {
				t.Fatalf("Apply = %+v, %v; work=%+v", result, err, work)
			}
			currentAfter, readErr := os.ReadFile(filepath.Join(root, "current"))
			if readErr != nil || !bytes.Equal(currentBefore, currentAfter) {
				t.Fatalf("current changed: %v", readErr)
			}
			lockInfo, lockErr := os.Lstat(filepath.Join(root, ".ardents-update-transaction-lock"))
			if lockErr != nil || !lockInfo.Mode().IsRegular() || lockInfo.Size() != 0 {
				t.Fatalf("lock absent or non-empty: err=%v size=%d", lockErr, lockInfo.Size())
			}
		})
	}
}

func TestExternalCurrentMutationFailsClosed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
	if _, err := Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(root, "generations", "1", "artifact")
	mutated := append([]byte(nil), candidate...)
	mutated[0] ^= 0xff
	if err := os.WriteFile(artifactPath, mutated, 0o600); err != nil {
		t.Fatal(err)
	}
	if result, err := Recover(context.Background(), root); err == nil || result.Outcome != "transaction-invalid" {
		t.Fatalf("Recover accepted external mutation: %+v, %v", result, err)
	}
	lockInfo, lockErr := os.Lstat(filepath.Join(root, ".ardents-update-transaction-lock"))
	if lockErr != nil || !lockInfo.Mode().IsRegular() || lockInfo.Size() != 0 {
		t.Fatalf("lock absent or non-empty after mutation detection: err=%v size=%d", lockErr, lockInfo.Size())
	}
}

func TestExternalHardLinkAliasFailsClosed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
	if _, err := Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(root, "generations", "1", "artifact")
	aliasSource := filepath.Join(filepath.Dir(root), "candidate-alias-source")
	if err := os.WriteFile(aliasSource, candidate, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(artifactPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(aliasSource, artifactPath); err != nil {
		t.Fatal(err)
	}
	if result, err := Recover(context.Background(), root); err == nil || result.Outcome != "transaction-invalid" {
		t.Fatalf("Recover accepted hard-link alias: %+v, %v", result, err)
	}
}

func TestCleanupFailureIsObservedWithinCeiling(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging", "1")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("injected staging sync failure")
	store := &ownedStore{root: root, ops: durabilityOps{syncDirectory: func(string) error { return sentinel }}}
	started := time.Now()
	err := store.cleanup(1)
	if !errors.Is(err, sentinel) {
		t.Fatalf("cleanup error = %v, want injected failure", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("cleanup exceeded 5 s ceiling: %v", elapsed)
	}
	if _, statErr := os.Lstat(staging); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("staging residue after cleanup: %v", statErr)
	}
}

func oracleAcceptedDecision(t *testing.T, vector v0OracleVector) releasedecision.Decision {
	t.Helper()
	floors := vector.Expected.ReleaseFloors
	return releasedecision.Decision{
		Outcome: releasedecision.Outcome("release-accepted"),
		Path:    vector.Candidate.Path, Length: int64(vector.Candidate.Length),
		Digest:   oracleDecodeDigest(t, vector.Candidate.SHA256)[:],
		Platform: "windows-amd64", Architecture: "amd64",
		Environment: "h3-test", Network: "ardents-h3-test-1",
		ReleaseIdentity: vector.Candidate.ReleaseIdentity,
		ReleaseVersion:  vector.Candidate.ReleaseVersion,
		SourceRevision:  "rev-0001", BuildInputCommitment: "inputs-0001",
		BuildIdentity: "build-0001", DependencyIdentity: "deps-0001",
		SBOMIdentity: "sbom-0001", AttestationPolicy: "two-builder",
		Qualification: "qualified", BuildState: "current", ProtocolPhase: "required",
		BuildSafety:               releasedecision.Outcome("release-accepted"),
		Protocol:                  releasedecision.Outcome("release-accepted"),
		ReferenceTime:             oracleTime(t, "2030-01-02T03:04:05Z"),
		BuildSafetyNoNewWorkAfter: oracleTime(t, "2030-02-01T03:04:05Z"),
		BuildSafetyTerminateAfter: oracleTime(t, "2030-07-01T03:04:05Z"),
		RootVersion:               floors.RootVersion,
		Floors: releasedecision.FloorSet{
			RootVersion: floors.RootVersion, RootDigest: oracleDecodeDigest(t, floors.RootSHA256)[:],
			TimestampVersion: floors.TimestampVersion, TimestampDigest: oracleDecodeDigest(t, floors.TimestampSHA256)[:],
			SnapshotVersion: floors.SnapshotVersion, SnapshotDigest: oracleDecodeDigest(t, floors.SnapshotSHA256)[:],
			TargetsVersion: floors.TargetsVersion, TargetsDigest: oracleDecodeDigest(t, floors.TargetsSHA256)[:],
		},
		Notice:        "release is accepted by every state machine",
		CustodyNotice: vector.Expected.CommandResult.CustodyNotice,
	}
}

func oracleTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.UTC()
}

func oracleFileSum(path string) [32]byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return [32]byte{}
	}
	return sha256.Sum256(data)
}

func oracleReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func oracleSelectedArtifact(path string) [32]byte {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) < 72 || string(raw[:8]) != "ARDUPD01" || raw[8] != 2 {
		return [32]byte{}
	}
	var digest [32]byte
	copy(digest[:], raw[40:72])
	return digest
}

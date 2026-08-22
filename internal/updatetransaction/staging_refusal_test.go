package updatetransaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestApplyRejectsCandidateMismatchBeforeRootInspection proves the first
// S7.2-03 refusal row through the public Apply boundary. The root does not
// exist: a mismatched candidate must not inspect or mutate owned state, and no
// work-control Adapter may run.
func TestApplyRejectsCandidateMismatchBeforeRootInspection(t *testing.T) {
	vector := oracleLoadV0(t)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	candidate[0] ^= 0xff
	work := &oracleWorkControl{}
	request := Request{
		UpdateRoot: filepath.Join(t.TempDir(), "uninspected-update-root"),
		Generation: vector.Request.TransactionGeneration,
		ActiveWork: vector.Request.ActiveWork,
		SchemaPlan: vector.Request.SchemaPlan,
		Decision:   oracleAcceptedDecision(t, vector),
		Artifact:   candidate,
		Work:       work,
		SelfTest:   oraclePassSelfTest{},
	}

	result, err := Apply(context.Background(), request)
	if err == nil {
		t.Fatal("Apply mismatch error = nil")
	}
	if result.Outcome != "staging-failed" || result.State != "release-accepted" {
		t.Fatalf("Apply mismatch outcome/state = %s/%s, want staging-failed/release-accepted", result.Outcome, result.State)
	}
	if result.Generation != vector.Request.TransactionGeneration || result.CurrentDigest != [32]byte{} ||
		result.RollbackDigest != [32]byte{} || result.StagingPresent || result.SafeNotice != "update staging failed" || result.CustodyNotice != "" {
		t.Fatalf("Apply mismatch result = %+v, want zero physical fields and fixed refusal notice", result)
	}
	if work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply mismatch called WorkControl: stop=%d drain=%d", work.stopCalls, work.drainCalls)
	}
}

// TestApplyRejectsCandidateLengthMismatchBeforeRootInspection proves the
// length half of the accepted candidate-mismatch refusal row.
func TestApplyRejectsCandidateLengthMismatchBeforeRootInspection(t *testing.T) {
	vector := oracleLoadV0(t)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	candidate = candidate[:len(candidate)-1]
	work := &oracleWorkControl{}
	request := Request{
		UpdateRoot: filepath.Join(t.TempDir(), "uninspected-update-root"),
		Generation: vector.Request.TransactionGeneration,
		ActiveWork: vector.Request.ActiveWork,
		SchemaPlan: vector.Request.SchemaPlan,
		Decision:   oracleAcceptedDecision(t, vector),
		Artifact:   candidate,
		Work:       work,
		SelfTest:   oraclePassSelfTest{},
	}

	result, err := Apply(context.Background(), request)
	if err == nil {
		t.Fatal("Apply length mismatch error = nil")
	}
	if result.Outcome != "staging-failed" || result.State != "release-accepted" ||
		result.SafeNotice != "update staging failed" || result.CustodyNotice != "" {
		t.Fatalf("Apply length mismatch result = %+v, want staging-failed/release-accepted without custody", result)
	}
	if result.CurrentDigest != [32]byte{} || result.RollbackDigest != [32]byte{} || result.StagingPresent ||
		work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply length mismatch made a physical or WorkControl claim: result=%+v work=%+v", result, work)
	}
}

// TestApplyRejectsFirstArtifactByteAboveCeiling proves the S7.2 resource
// ceiling wins before root inspection even when an otherwise accepted Decision
// is forged to declare the first byte above the fixed payload limit.
func TestApplyRejectsFirstArtifactByteAboveCeiling(t *testing.T) {
	vector := oracleLoadV0(t)
	decision := oracleAcceptedDecision(t, vector)
	decision.Length = maximumArtifactBytes + 1
	work := &oracleWorkControl{}
	request := Request{
		UpdateRoot: filepath.Join(t.TempDir(), "uninspected-update-root"),
		Generation: vector.Request.TransactionGeneration,
		ActiveWork: vector.Request.ActiveWork,
		SchemaPlan: vector.Request.SchemaPlan,
		Decision:   decision,
		Artifact:   oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256),
		Work:       work,
		SelfTest:   oraclePassSelfTest{},
	}

	result, err := Apply(context.Background(), request)
	if err == nil {
		t.Fatal("Apply oversized error = nil")
	}
	if result.Outcome != "resource-denied" || result.State != "release-accepted" {
		t.Fatalf("Apply oversized outcome/state = %s/%s, want resource-denied/release-accepted", result.Outcome, result.State)
	}
	if result.Generation != vector.Request.TransactionGeneration || result.CurrentDigest != [32]byte{} ||
		result.RollbackDigest != [32]byte{} || result.StagingPresent || result.SafeNotice != "update resources unavailable" || result.CustodyNotice != "" {
		t.Fatalf("Apply oversized result = %+v, want zero physical fields and fixed resource notice", result)
	}
	if work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply oversized called WorkControl: stop=%d drain=%d", work.stopCalls, work.drainCalls)
	}
}

// TestApplyRefusesOneByteBelowObservedEnvelope keeps the resource boundary at
// the private observation seam: the test supplies literal capacity values and
// observes the public Result, not production arithmetic internals.
func TestApplyRefusesOneByteBelowObservedEnvelope(t *testing.T) {
	root, request := applyCheckpointRequest(t)
	_ = root
	work := request.Work.(*oracleWorkControl)
	result, err := applyWithResourceObservation(context.Background(), request, resourceObservation{
		allocationUnit: 1,
		availableBytes: 0,
		availableItems: resourceObjectCount,
		itemsKnown:     true,
	})
	if err == nil {
		t.Fatal("Apply insufficient envelope error = nil")
	}
	if result.Outcome != "resource-denied" || result.State != "release-accepted" ||
		result.Generation != request.Generation || result.CurrentDigest != [32]byte{} ||
		result.RollbackDigest != [32]byte{} || result.StagingPresent ||
		result.SafeNotice != "update resources unavailable" || result.CustodyNotice != "" {
		t.Fatalf("Apply insufficient envelope result = %+v, want public resource refusal", result)
	}
	if work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply insufficient envelope called WorkControl: %#v", work)
	}
	for _, path := range []string{filepath.Join(root, "staging", "1"), filepath.Join(root, "staging", "1.tmp"), filepath.Join(root, "transactions", "1")} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Apply insufficient envelope created %s: %v", path, statErr)
		}
	}
}

// TestApplyReportsAndCleansStageWriteFailure proves a declared incomplete
// staging tree is cleaned before the public refusal is returned. The injected
// private operation is limited to candidate-file creation; no root inspection,
// Result mapping, or cleanup policy is replaced.
func TestApplyReportsAndCleansStageWriteFailure(t *testing.T) {
	root, request := applyCheckpointRequest(t)
	currentBefore := oracleReadFile(t, filepath.Join(root, "current"))
	activeBefore := oracleReadFile(t, filepath.Join(root, "generations", "0", "artifact"))
	operations := nativeStageOperations(nativeDurability())
	sentinel := errors.New("injected candidate write failure")
	operations.openFile = func(string) (stageFile, error) { return nil, sentinel }

	result, err := applyWithStageOperations(context.Background(), request, operations)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Apply stage write error = %v, want injected error", err)
	}
	if result.Outcome != "staging-failed" || result.State != "artifact-verified" ||
		result.Generation != request.Generation || result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) ||
		result.RollbackDigest != [32]byte{} || result.StagingPresent || result.SafeNotice != "update staging failed" ||
		result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Apply stage write result = %+v, want staging-failed/artifact-verified with predecessor evidence", result)
	}
	if work, ok := request.Work.(*oracleWorkControl); !ok || work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply stage write called WorkControl: %#v", request.Work)
	}
	if currentAfter := oracleReadFile(t, filepath.Join(root, "current")); string(currentAfter) != string(currentBefore) ||
		string(oracleReadFile(t, filepath.Join(root, "generations", "0", "artifact"))) != string(activeBefore) {
		t.Fatal("Apply stage write changed active selection or predecessor payload")
	}
	for _, path := range []string{filepath.Join(root, "staging", "1"), filepath.Join(root, "staging", "1.tmp"), filepath.Join(root, "transactions", "1")} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Apply stage write left declared incomplete path %s: %v", path, statErr)
		}
	}
}

// TestApplyRefusesCoherentOccupiedStaging proves a complete interrupted
// candidate owns the staging slot. Apply reports it without calling recovery,
// cleaning it, or admitting any new work.
func TestApplyRefusesCoherentOccupiedStaging(t *testing.T) {
	root, request := applyCheckpointRequest(t)
	predecessorRaw := oracleReadFile(t, filepath.Join(root, "current"))
	predecessor := recoveryOraclePredecessorEnvelope(t, sha256.Sum256(predecessorRaw),
		recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleBootstrapManifestDigest(t, root),
		recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleBootstrapManifestDigest(t, root))
	artifact, manifest := recoveryOracleStage(t, root, request.Generation)
	recoveryOracleWriteChain(t, root, request.Generation, predecessor, artifact, manifest, byte(stateStaged))
	before := recoveryOracleTreeDigest(t, root)

	result, err := Apply(context.Background(), request)
	if err == nil {
		t.Fatal("Apply occupied staging error = nil")
	}
	if result.Outcome != "resource-denied" || result.State != "staged" || result.Generation != request.Generation ||
		result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) || result.RollbackDigest != [32]byte{} ||
		!result.StagingPresent || result.SafeNotice != "update recovery required" || result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Apply occupied staging result = %+v", result)
	}
	if work := request.Work.(*oracleWorkControl); work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply occupied staging called WorkControl: %#v", work)
	}
	if after := recoveryOracleTreeDigest(t, root); !bytes.Equal(after, before) {
		t.Fatal("Apply occupied staging mutated recovery-owned evidence")
	}
}

type stagingFaultFile struct {
	file       stageFile
	writeErr   error
	syncErr    error
	closeErr   error
	shortWrite bool
}

func (file stagingFaultFile) Write(data []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	if file.shortWrite {
		return len(data) - 1, nil
	}
	return file.file.Write(data)
}
func (file stagingFaultFile) Sync() error {
	if file.syncErr != nil {
		return file.syncErr
	}
	return file.file.Sync()
}
func (file stagingFaultFile) Close() error {
	closeErr := file.file.Close()
	if file.closeErr != nil {
		return errors.Join(file.closeErr, closeErr)
	}
	return closeErr
}

// TestApplyCleansEveryStageOperationFailure exercises each S7.2-03 operation
// failure without replacing any recovery or Result policy.
func TestApplyCleansEveryStageOperationFailure(t *testing.T) {
	for _, row := range []struct {
		name  string
		apply func(stageOperations, error) stageOperations
	}{
		{"short-write", func(operations stageOperations, sentinel error) stageOperations {
			nativeOpen := operations.openFile
			operations.openFile = func(path string) (stageFile, error) {
				file, err := nativeOpen(path)
				return stagingFaultFile{file: file, shortWrite: true}, err
			}
			return operations
		}},
		{"flush", func(operations stageOperations, sentinel error) stageOperations {
			nativeOpen := operations.openFile
			operations.openFile = func(path string) (stageFile, error) {
				file, err := nativeOpen(path)
				return stagingFaultFile{file: file, syncErr: sentinel}, err
			}
			return operations
		}},
		{"close", func(operations stageOperations, sentinel error) stageOperations {
			nativeOpen := operations.openFile
			operations.openFile = func(path string) (stageFile, error) {
				file, err := nativeOpen(path)
				return stagingFaultFile{file: file, closeErr: sentinel}, err
			}
			return operations
		}},
		{"rename", func(operations stageOperations, sentinel error) stageOperations {
			operations.renameDirectory = func(string, string) error { return sentinel }
			return operations
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			root, request := applyCheckpointRequest(t)
			sentinel := errors.New("injected " + row.name + " failure")
			result, err := applyWithStageOperations(context.Background(), request, row.apply(nativeStageOperations(nativeDurability()), sentinel))
			if row.name == "short-write" {
				if err == nil {
					t.Fatal("short write error = nil")
				}
			} else if !errors.Is(err, sentinel) {
				t.Fatalf("Apply %s error = %v, want sentinel", row.name, err)
			}
			if result.Outcome != "staging-failed" || result.State != "artifact-verified" || result.Generation != request.Generation ||
				result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) || result.RollbackDigest != [32]byte{} ||
				result.StagingPresent || result.SafeNotice != "update staging failed" || result.CustodyNotice != recoveryOracleCustodyNotice {
				t.Fatalf("Apply %s result = %+v", row.name, result)
			}
			if work := request.Work.(*oracleWorkControl); work.stopCalls != 0 || work.drainCalls != 0 {
				t.Fatalf("Apply %s called WorkControl: %#v", row.name, work)
			}
			for _, path := range []string{filepath.Join(root, "staging", "1"), filepath.Join(root, "staging", "1.tmp"), filepath.Join(root, "transactions", "1")} {
				if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("Apply %s retained %s: %v", row.name, path, statErr)
				}
			}
		})
	}
}

func TestApplyCleansStageParentAcknowledgementFailure(t *testing.T) {
	root, request := applyCheckpointRequest(t)
	sentinel := errors.New("injected acknowledgement failure")
	operations := nativeStageOperations(nativeDurability())
	operations.acknowledge = func(string) error { return sentinel }
	result, err := applyWithStageOperations(context.Background(), request, operations)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Apply acknowledgement error = %v, want sentinel", err)
	}
	if result.Outcome != "staging-failed" || result.State != "artifact-verified" || result.Generation != request.Generation ||
		result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) || result.RollbackDigest != [32]byte{} ||
		result.StagingPresent || result.SafeNotice != "update staging failed" || result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Apply acknowledgement result = %+v", result)
	}
	if work := request.Work.(*oracleWorkControl); work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply acknowledgement called WorkControl: %#v", work)
	}
	for _, path := range []string{filepath.Join(root, "staging", "1"), filepath.Join(root, "staging", "1.tmp"), filepath.Join(root, "transactions", "1")} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Apply acknowledgement retained %s: %v", path, statErr)
		}
	}
}

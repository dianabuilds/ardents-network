package update

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

type retrySelfTest struct {
	calls uint64
}

type failedSelfTest struct{}

func (failedSelfTest) Check(context.Context, CandidateIdentity) error {
	return errors.New("local self-test failure")
}

func oracleRollbackDecision(t *testing.T, vector v0OracleVector) release.Decision {
	t.Helper()
	manifest := vector.Initial.ActivePayload.Manifest
	floors := vector.Expected.ReleaseFloors
	return release.Decision{Outcome: "release-accepted", Path: manifest.TargetPath, Length: int64(vector.Initial.ActivePayload.Length),
		Digest: oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256)[:], Platform: manifest.Platform,
		Architecture: manifest.Architecture, Environment: manifest.Environment, Network: manifest.Network,
		ReleaseIdentity: manifest.ReleaseIdentity, ReleaseVersion: int64(manifest.ReleaseVersion), SourceRevision: manifest.SourceRevision,
		BuildInputCommitment: manifest.BuildInputCommitment, BuildIdentity: manifest.BuildIdentity,
		DependencyIdentity: manifest.DependencyIdentity, SBOMIdentity: manifest.SBOMIdentity,
		AttestationPolicy: manifest.AttestationPolicy, Qualification: manifest.Qualification, BuildState: manifest.BuildState,
		ProtocolPhase: manifest.ProtocolPhase, BuildSafety: "release-accepted", Protocol: "release-accepted",
		ReferenceTime: oracleTime(t, manifest.ReferenceTime), BuildSafetyNoNewWorkAfter: oracleTime(t, manifest.BuildSafetyNoNewWorkAfter),
		BuildSafetyTerminateAfter: oracleTime(t, manifest.BuildSafetyTerminateAfter), RootVersion: floors.RootVersion,
		Floors: release.FloorSet{RootVersion: floors.RootVersion, RootDigest: oracleDecodeDigest(t, floors.RootSHA256)[:],
			TimestampVersion: floors.TimestampVersion, TimestampDigest: oracleDecodeDigest(t, floors.TimestampSHA256)[:],
			SnapshotVersion: floors.SnapshotVersion, SnapshotDigest: oracleDecodeDigest(t, floors.SnapshotSHA256)[:],
			TargetsVersion: floors.TargetsVersion, TargetsDigest: oracleDecodeDigest(t, floors.TargetsSHA256)[:]},
		EvidenceNotice: manifest.EvidenceNotice}
}

func (test *retrySelfTest) Check(context.Context, CandidateIdentity) error {
	test.calls++
	if test.calls == 1 {
		return ErrSelfTestUnavailable
	}
	return nil
}

func TestSelfTestFailureBecomesRollbackPending(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	work := &oracleWorkControl{}
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "no-op-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: work, SelfTest: failedSelfTest{}}
	result, err := Apply(context.Background(), request)
	if err == nil || result.Outcome != "self-test-failed" || result.State != "rollback-pending" ||
		result.Generation != 1 || result.CurrentDigest != *oracleDecodeDigest(t, vector.Candidate.SHA256) ||
		result.RollbackDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) || result.StagingPresent ||
		result.SafeNotice != "update self-test failed" {
		t.Fatalf("Apply = %+v, %v", result, err)
	}
	if work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("work calls = %#v", work)
	}
	repeated, repeatedErr := Apply(context.Background(), request)
	if !errors.Is(repeatedErr, errRollbackPending) || repeated != result || work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("pending Apply = %+v, %v; work=%#v", repeated, repeatedErr, work)
	}
	refusedRequest := request
	refusedRequest.rollbackDecision = request.decision
	refused, refusedErr := Apply(context.Background(), refusedRequest)
	if !errors.Is(refusedErr, errRollbackRefused) || refused.Outcome != "rollback-refused" || refused.State != "repair-required" ||
		refused.CurrentDigest != result.CurrentDigest || refused.RollbackDigest != result.RollbackDigest ||
		refused.SafeNotice != "update rollback refused" || work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("refused Apply = %+v, %v; work=%#v", refused, refusedErr, work)
	}
	entries, readErr := os.ReadDir(filepath.Join(root, "transactions", "1", "journal"))
	if readErr != nil || len(entries) != 10 || entries[9].Name() != "12-repair-required.entry" {
		t.Fatalf("rollback-pending journal = %v, %v", entries, readErr)
	}
	recovered, recoveryErr := Recover(context.Background(), root)
	if !errors.Is(recoveryErr, errRollbackRefused) || recovered.Outcome != "rollback-refused" || recovered.State != "repair-required" ||
		recovered.CurrentDigest != result.CurrentDigest || recovered.RollbackDigest != result.RollbackDigest {
		t.Fatalf("Recover = %+v, %v", recovered, recoveryErr)
	}
}

func TestRollbackToVerifiedPredecessor(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	work := &oracleWorkControl{}
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "no-op-v1", decision: oracleAcceptedDecision(t, vector),
		Artifact: candidate, Work: work, SelfTest: failedSelfTest{}}
	if _, err := Apply(context.Background(), request); err == nil {
		t.Fatal("local self-test failure was accepted")
	}
	request.rollbackDecision = oracleRollbackDecision(t, vector)
	request.SelfTest = oraclePassSelfTest{}
	result, err := Apply(context.Background(), request)
	if !errors.Is(err, errRolledBack) || result.Outcome != "rolled-back" || result.State != "rolled-back" ||
		result.CurrentDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) || result.RollbackDigest != [32]byte{} ||
		result.SafeNotice != "update rolled back" || work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("rollback Apply = %+v, %v; work=%#v", result, err, work)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "generations", "1")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed candidate remains: %v", statErr)
	}
	recovered, recoveryErr := Recover(context.Background(), root)
	if !errors.Is(recoveryErr, errRolledBack) || recovered != result {
		t.Fatalf("rollback Recover = %+v, %v; want %+v", recovered, recoveryErr, result)
	}
}

func TestRollbackSelfTestFailureRequiresRepair(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	work := &oracleWorkControl{}
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "no-op-v1", decision: oracleAcceptedDecision(t, vector),
		Artifact: candidate, Work: work, SelfTest: failedSelfTest{}}
	if _, err := Apply(context.Background(), request); err == nil {
		t.Fatal("initial failed self-test accepted")
	}
	request.rollbackDecision = oracleRollbackDecision(t, vector)
	result, err := Apply(context.Background(), request)
	if !errors.Is(err, errRepairRequired) || result.Outcome != "repair-required" || result.State != "repair-required" ||
		result.CurrentDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) || result.RollbackDigest != [32]byte{} ||
		work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("repair Apply = %+v, %v; work=%#v", result, err, work)
	}
	recovered, recoveryErr := Recover(context.Background(), root)
	if !errors.Is(recoveryErr, errRepairRequired) || recovered != result {
		t.Fatalf("repair Recover = %+v, %v; want %+v", recovered, recoveryErr, result)
	}
}

func TestSelfTestUnavailableRetry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	work := &oracleWorkControl{}
	selfTest := &retrySelfTest{}
	request := Request{UpdateRoot: root, generation: 1, schemaPlan: "no-op-v1",
		decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: work, SelfTest: selfTest}
	first, firstErr := Apply(context.Background(), request)
	if !errors.Is(firstErr, ErrSelfTestUnavailable) || first.Outcome != "application-networking-unverified" ||
		first.State != "self-testing" || first.Generation != 1 || first.CurrentDigest != *oracleDecodeDigest(t, vector.Candidate.SHA256) ||
		first.RollbackDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) || first.StagingPresent ||
		first.SafeNotice != "update networking unverified" || first.EvidenceNotice != vector.Expected.CommandResult.EvidenceNotice {
		t.Fatalf("first Apply = %+v, %v", first, firstErr)
	}
	if work.stopCalls != 1 || work.drainCalls != 1 || selfTest.calls != 1 {
		t.Fatalf("first Adapter calls work=%#v self=%d", work, selfTest.calls)
	}
	second, secondErr := Apply(context.Background(), request)
	if secondErr != nil || second.Outcome != "committed" || second.State != "committed" || second.Generation != 1 ||
		second.CurrentDigest != *oracleDecodeDigest(t, vector.Candidate.SHA256) ||
		second.RollbackDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) {
		t.Fatalf("retry Apply = %+v, %v", second, secondErr)
	}
	if work.stopCalls != 1 || work.drainCalls != 1 || selfTest.calls != 2 {
		t.Fatalf("retry repeated work control: work=%#v self=%d", work, selfTest.calls)
	}
}

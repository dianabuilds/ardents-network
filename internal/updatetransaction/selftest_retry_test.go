package updatetransaction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type retrySelfTest struct {
	calls uint64
}

type failedSelfTest struct{}

func (failedSelfTest) Check(context.Context, CandidateIdentity) error {
	return errors.New("local self-test failure")
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
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: work, SelfTest: failedSelfTest{}}
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
	entries, readErr := os.ReadDir(filepath.Join(root, "transactions", "1", "journal"))
	if readErr != nil || len(entries) != 9 || entries[8].Name() != "10-rollback-pending.entry" {
		t.Fatalf("rollback-pending journal = %v, %v", entries, readErr)
	}
	recovered, recoveryErr := Recover(context.Background(), root)
	if recoveryErr != nil || recovered.Outcome != "recovered" || recovered.State != "rollback-pending" ||
		recovered.CurrentDigest != result.CurrentDigest || recovered.RollbackDigest != result.RollbackDigest {
		t.Fatalf("Recover = %+v, %v", recovered, recoveryErr)
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
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: work, SelfTest: selfTest}
	first, firstErr := Apply(context.Background(), request)
	if !errors.Is(firstErr, ErrSelfTestUnavailable) || first.Outcome != "application-networking-unverified" ||
		first.State != "self-testing" || first.Generation != 1 || first.CurrentDigest != *oracleDecodeDigest(t, vector.Candidate.SHA256) ||
		first.RollbackDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) || first.StagingPresent ||
		first.SafeNotice != "update networking unverified" || first.CustodyNotice != vector.Expected.CommandResult.CustodyNotice {
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

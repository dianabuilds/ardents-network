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

func (test *retrySelfTest) Check(context.Context, CandidateIdentity) error {
	test.calls++
	if test.calls == 1 {
		return ErrSelfTestUnavailable
	}
	return nil
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

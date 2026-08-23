package update

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestApplyRejectsUninitializedRootBeforeWorkAdapters proves that root
// admission is update-owned and happens before an injected runtime Adapter.
func TestApplyRejectsUninitializedRootBeforeWorkAdapters(t *testing.T) {
	vector := oracleLoadV0(t)
	work := &oracleWorkControl{}
	result, err := Apply(context.Background(), Request{
		UpdateRoot: filepath.Join(t.TempDir(), "uninspected-update-root"),
		generation: vector.Request.TransactionGeneration, schemaPlan: vector.Request.SchemaPlan,
		decision: oracleAcceptedDecision(t, vector), Artifact: oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256),
		Work: work, SelfTest: oraclePassSelfTest{},
	})
	if err == nil || result.Outcome != invalidOutcome || result.State != "release-accepted" {
		t.Fatalf("Apply uninitialized-root refusal result=%+v err=%v", result, err)
	}
	if work.stopCalls != 0 || work.drainCalls != 0 {
		t.Fatalf("Apply active-work refusal called work control: %#v", work)
	}
}

type drainRefusalWorkControl struct {
	stopErr, drainErr           error
	stopCalls, drainCalls       uint64
	stopDeadline, drainDeadline time.Time
}

func (control *drainRefusalWorkControl) StopNewWork(ctx context.Context) error {
	control.stopCalls++
	control.stopDeadline, _ = ctx.Deadline()
	return control.stopErr
}

func (control *drainRefusalWorkControl) Drain(ctx context.Context) error {
	control.drainCalls++
	control.drainDeadline, _ = ctx.Deadline()
	return control.drainErr
}

func (*drainRefusalWorkControl) StopNewAssignments(context.Context) error { return nil }
func (*drainRefusalWorkControl) DrainAssignments(context.Context) error   { return nil }
func (*drainRefusalWorkControl) RejoinOrWithdraw(context.Context) error   { return nil }

func TestDrainRefusal(t *testing.T) {
	tests := []struct {
		name       string
		stopErr    error
		drainErr   error
		wantState  string
		wantDrains uint64
	}{
		{name: "stop", stopErr: errors.New("stop refused"), wantState: "rollback-reserved"},
		{name: "drain", drainErr: errors.New("drain refused"), wantState: "stop-new-work", wantDrains: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "update")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			vector := oracleBootstrapV0(t, root)
			candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
			currentBefore := oracleReadFile(t, filepath.Join(root, "current"))
			work := &drainRefusalWorkControl{stopErr: test.stopErr, drainErr: test.drainErr}
			request := Request{UpdateRoot: root, generation: 1,
				schemaPlan: "no-op-v1", decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
				Work: work, SelfTest: oraclePassSelfTest{}}
			result, err := Apply(context.Background(), request)
			if err == nil || result.Outcome != "drain-expired" || result.State != test.wantState ||
				result.Generation != 1 || result.CurrentDigest != *oracleDecodeDigest(t, vector.Initial.ActivePayload.SHA256) ||
				result.RollbackDigest != [32]byte{} || result.StagingPresent || result.SafeNotice != "update drain expired" {
				t.Fatalf("Apply = %+v, %v; work=%#v", result, err, work)
			}
			if work.stopCalls != 1 || work.drainCalls != test.wantDrains {
				t.Fatalf("work calls = %#v", work)
			}
			if !bytes.Equal(currentBefore, oracleReadFile(t, filepath.Join(root, "current"))) {
				t.Fatal("bounded refusal changed current")
			}
			for _, path := range []string{filepath.Join(root, "staging", "1"), filepath.Join(root, "transactions", "1")} {
				if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("bounded refusal retained %s: %v", path, statErr)
				}
			}
		})
	}
}

func TestDrainDeadline(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	work := &drainRefusalWorkControl{}
	start := time.Now().Add(2 * time.Second)
	result, err := applyWithStart(context.Background(), Request{UpdateRoot: root, generation: 1,
		schemaPlan: "no-op-v1", decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: work, SelfTest: oraclePassSelfTest{}}, start)
	if err != nil || result.Outcome != "committed" || work.stopCalls != 1 || work.drainCalls != 1 {
		t.Fatalf("Apply = %+v, %v; work=%#v", result, err, work)
	}
	want := start.Add(15 * time.Second)
	if !work.stopDeadline.Equal(want) || !work.drainDeadline.Equal(want) {
		t.Fatalf("deadlines stop=%s drain=%s, want %s", work.stopDeadline, work.drainDeadline, want)
	}
}

func TestBoundedCallRejectsLateSuccess(t *testing.T) {
	called := false
	err := callBounded(context.Background(), time.Now().Add(-time.Second), func(context.Context) error {
		called = true
		return nil
	})
	if !called || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("late successful call = %v, called=%t", err, called)
	}
}

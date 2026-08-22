package updatetransaction

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type contributorWorkProbe struct {
	oracleWorkControl
	stopAssignments, drainAssignments, rejoinOrWithdraw uint64
}

func (probe *contributorWorkProbe) StopNewAssignments(context.Context) error {
	probe.stopAssignments++
	return nil
}

func (probe *contributorWorkProbe) DrainAssignments(context.Context) error {
	probe.drainAssignments++
	return nil
}

func (probe *contributorWorkProbe) RejoinOrWithdraw(context.Context) error {
	probe.rejoinOrWithdraw++
	return nil
}

func TestContributorWorkRunsInUpdateOrder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	work := &contributorWorkProbe{}
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: work, SelfTest: oraclePassSelfTest{}}
	result, err := Apply(context.Background(), request)
	if err != nil || result.Outcome != "committed" || work.stopCalls != 1 || work.drainCalls != 1 ||
		work.stopAssignments != 1 || work.drainAssignments != 1 || work.rejoinOrWithdraw != 1 {
		t.Fatalf("Apply = %+v, %v; contributor work=%+v", result, err, work)
	}
}

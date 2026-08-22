package updatetransaction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type schemaContractProbe struct{ calls uint64 }

func (probe *schemaContractProbe) Plan(context.Context, uint64, string, SchemaSelection) (SchemaSelection, bool, error) {
	probe.calls++
	return SchemaSelection{}, false, errors.New("unexpected schema plan")
}

func (probe *schemaContractProbe) Prepare(context.Context, SchemaSelection) error {
	probe.calls++
	return errors.New("unexpected schema prepare")
}

func (probe *schemaContractProbe) Inspect(context.Context, SchemaSelection) error {
	probe.calls++
	return errors.New("unexpected schema inspect")
}

func (probe *schemaContractProbe) Discard(context.Context, SchemaSelection) error {
	probe.calls++
	return errors.New("unexpected schema discard")
}

func TestCopyOnWritePlanIsRejectedBeforeAdapterOrRootMutation(t *testing.T) {
	vector := oracleLoadV0(t)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	probe := &schemaContractProbe{}
	request := Request{UpdateRoot: filepath.Join(t.TempDir(), "not-created"), Generation: 1,
		SchemaPlan: "copy-on-write-v1", Decision: oracleAcceptedDecision(t, vector), Artifact: candidate,
		Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}, Schema: probe}

	result, err := Apply(context.Background(), request)
	if !errors.Is(err, errRecordInvalid) || result.Outcome != invalidOutcome || result.State != "release-accepted" || probe.calls != 0 {
		t.Fatalf("Apply = %+v, %v; schema calls=%d", result, err, probe.calls)
	}
}

func TestNoOpSchemaDoesNotInvokeAdapter(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	probe := &schemaContractProbe{}
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{},
		SelfTest: oraclePassSelfTest{}, Schema: probe}

	if _, err := Apply(context.Background(), request); err != nil {
		t.Fatalf("Apply no-op schema: %v", err)
	}
	if probe.calls != 0 {
		t.Fatalf("no-op invoked schema adapter %d times", probe.calls)
	}
}

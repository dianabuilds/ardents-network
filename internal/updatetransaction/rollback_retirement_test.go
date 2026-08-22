package updatetransaction

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestApplyRotatesVerifiedRollbackForNextGeneration(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	first := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
	if result, err := Apply(context.Background(), first); err != nil || result.Outcome != "committed" {
		t.Fatalf("first Apply = %+v, %v", result, err)
	}
	second := first
	second.Generation = 2
	second.Work = &oracleWorkControl{}
	if result, err := Apply(context.Background(), second); err != nil || result.Outcome != "committed" || result.Generation != 2 {
		rootEntries, _ := os.ReadDir(root)
		generationEntries, _ := os.ReadDir(filepath.Join(root, "generations"))
		transactionEntries, _ := os.ReadDir(filepath.Join(root, "transactions"))
		t.Fatalf("second Apply = %+v, %v; root=%v generations=%v transactions=%v", result, err, rootEntries, generationEntries, transactionEntries)
	}
	entries, err := os.ReadDir(filepath.Join(root, "generations"))
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !reflect.DeepEqual(names, []string{"1", "2"}) {
		t.Fatalf("retained generations = %v, want [1 2]", names)
	}
	raw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	selection, err := decodeCurrent(raw)
	if err != nil || selection.Transaction != 2 || selection.Rollback == nil || selection.Rollback.Generation != 1 {
		t.Fatalf("current after rotation = %+v, %v", selection, err)
	}
}

func TestRecoverCompletesBoundedRollbackRetirement(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
	if _, err := Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	marker, err := encodeRollbackRetirement(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rollbackRetireName), marker, 0o600); err != nil {
		t.Fatal(err)
	}
	result, recoverErr := Recover(context.Background(), root)
	if recoverErr != nil || result.Outcome != "recovered" || result.State != "idle" || result.Generation != 1 {
		t.Fatalf("Recover = %+v, %v", result, recoverErr)
	}
	for _, path := range []string{filepath.Join(root, rollbackRetireName), filepath.Join(root, "generations", "0"), filepath.Join(root, "transactions", "1")} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("retirement recovery retained %s: %v", path, statErr)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	selection, err := decodeCurrent(raw)
	if err != nil || selection.Transaction != 1 || selection.Rollback != nil {
		t.Fatalf("retired current = %+v, %v", selection, err)
	}
}

func TestRecoverRemovesBoundRetirementCurrentTemp(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	request := Request{UpdateRoot: root, Generation: 1, SchemaPlan: "no-op-v1",
		Decision: oracleAcceptedDecision(t, vector), Artifact: candidate, Work: &oracleWorkControl{}, SelfTest: oraclePassSelfTest{}}
	if _, err := Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	marker, err := encodeRollbackRetirement(previous)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := decodeCurrent(previous)
	if err != nil {
		t.Fatal(err)
	}
	withoutRollback, err := encodeCurrent(currentSelection{Transaction: selection.Transaction, Current: selection.Current})
	if err != nil {
		t.Fatal(err)
	}
	temp := ".current.0123456789abcdef.tmp"
	if err := os.WriteFile(filepath.Join(root, rollbackRetireName), marker, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, temp), withoutRollback, 0o600); err != nil {
		t.Fatal(err)
	}
	result, recoverErr := Recover(context.Background(), root)
	if recoverErr != nil || result.Outcome != "recovered" || result.State != "idle" {
		t.Fatalf("Recover = %+v, %v", result, recoverErr)
	}
	if _, statErr := os.Lstat(filepath.Join(root, temp)); !os.IsNotExist(statErr) {
		t.Fatalf("retirement recovery retained current temp: %v", statErr)
	}
}

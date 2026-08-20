package stage6verify_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func TestStage6VerifierRejectsUnownedTraceFields(t *testing.T) {
	root := t.TempDir()
	writeEvidenceCampaign(t, root, "source-commit", "clean")
	if err := committedTraceMutation(root, 0, func(trace *mutationTrace) {
		trace.Values = []int64{1}
		trace.Fields = []string{"candidate-authored"}
	}); err != nil {
		t.Fatal(err)
	}
	verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
	if verdict.Status != "fail" || len(verdict.Diagnostics) != 1 || verdict.Diagnostics[0] != "A0:predicate-false" {
		t.Fatalf("verdict=%+v", verdict)
	}
	if err := committedTraceMutation(root, 1, func(trace *mutationTrace) {
		trace.Auxiliary = []byte("candidate-authored")
		trace.Values = []int64{1}
		trace.Fields = []string{"candidate-authored"}
	}); err != nil {
		t.Fatal(err)
	}
	verdict = (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict-a1"))
	if verdict.Status != "fail" || len(verdict.Diagnostics) != 2 ||
		verdict.Diagnostics[0] != "A0:predicate-false" || verdict.Diagnostics[1] != "A1:predicate-false" {
		t.Fatalf("verdict=%+v", verdict)
	}
}

func TestStage6VerifierClassifiesEveryCellBehaviorMutationAsFail(t *testing.T) {
	base := t.TempDir()
	writeEvidenceCampaign(t, base, "source-commit", "clean")
	cellIDs := []string{"A0", "A1", "A2", "A3", "A4", "A5", "B0", "B1", "B2", "B3", "B4", "B5",
		"C0", "C1", "C2", "C3", "C4", "C5", "C6", "C7", "D0", "D1", "D2", "D3", "D4", "D5", "D6"}
	for ordinal, cellID := range cellIDs {
		t.Run(cellID, func(t *testing.T) {
			root := t.TempDir()
			if err := cloneBundle(base, root); err != nil {
				t.Fatal(err)
			}
			if err := committedTraceMutation(root, ordinal, func(trace *mutationTrace) {
				trace.Operation = "mutated-operation"
			}); err != nil {
				t.Fatal(err)
			}
			verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
				filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
			if verdict.Status != "fail" || len(verdict.Diagnostics) != 1 ||
				verdict.Diagnostics[0] != cellID+":predicate-false" {
				t.Fatalf("verdict=%+v", verdict)
			}
		})
	}
	root := t.TempDir()
	if err := cloneBundle(base, root); err != nil {
		t.Fatal(err)
	}
	if err := terminalMutation(func(terminal *mutationTerminal) {
		terminal.Class = "different-runtime-class"
	})(root); err != nil {
		t.Fatal(err)
	}
	if err := rewriteIndex(root, func(index *mutationIndex) {
		index.Cells[0].TerminalClass = "different-runtime-class"
	}); err != nil {
		t.Fatal(err)
	}
	verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
	if verdict.Status != "fail" || len(verdict.Diagnostics) != 1 || verdict.Diagnostics[0] != "A0:predicate-false" {
		t.Fatalf("terminal class verdict=%+v", verdict)
	}
}

func committedTraceMutation(root string, ordinal int, mutate func(*mutationTrace)) error {
	prefix := twoDigitOrdinal(ordinal)
	path := filepath.Join(root, "evidence", "cells", prefix, "observations", "trace.jsonl")
	var trace mutationTrace
	if err := readMutationJSONL(path, &trace); err != nil {
		return err
	}
	mutate(&trace)
	if err := writeMutationJSON(path, trace, true); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	return rewriteIndex(root, func(index *mutationIndex) {
		stream := &index.Cells[ordinal].Streams[0]
		stream.Size = int64(len(raw))
		stream.SHA256 = hex.EncodeToString(digest[:])
	})
}

func twoDigitOrdinal(value int) string {
	return string([]byte{'0' + byte(value/10), '0' + byte(value%10)})
}

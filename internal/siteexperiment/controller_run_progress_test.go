package siteexperiment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyAttemptResultPreservesTimeoutAndCleanupFailure(t *testing.T) {
	parent := t.Context()
	attempt, cancel := context.WithCancel(parent)
	cancel()
	cleanupErr := errors.New("cleanup failed")
	err := classifyAttemptResult(parent, attempt, cleanupErr)
	if !errors.Is(err, errMatrixOperational) || !errors.Is(err, context.Canceled) || !errors.Is(err, cleanupErr) {
		t.Fatalf("attempt error lost context or cleanup cause: %v", err)
	}
}

func TestClassifyAttemptResultLeavesScenarioFailureTerminal(t *testing.T) {
	want := scenarioFailure(errors.New("workload mismatch"))
	if got := classifyAttemptResult(t.Context(), t.Context(), want); !errors.Is(got, errScenarioFailure) || errors.Is(got, errMatrixOperational) {
		t.Fatalf("scenario result was misclassified: %v", got)
	}
}

func TestWriteRunProgress(t *testing.T) {
	directory := t.TempDir()
	if err := writeRunProgress(directory, "running-attempt", 7); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "progress.json"))
	if err != nil {
		t.Fatal(err)
	}
	var progress runProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		t.Fatal(err)
	}
	if progress.SchemaVersion != "gatec-progress/v1" || progress.Stage != "running-attempt" || progress.Attempt != 7 {
		t.Fatalf("unexpected progress evidence: %+v", progress)
	}
}

func TestWriteRunProgressRejectsMissingEvidenceDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	if err := writeRunProgress(path, "preparing", 0); err == nil {
		t.Fatal("expected missing evidence directory to fail")
	}
}

func TestWriteInterruptionEvidence(t *testing.T) {
	directory := t.TempDir()
	if err := writeInterruptionEvidence(directory, "lifecycle-timeout", 4); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "interruption.json"))
	if err != nil {
		t.Fatal(err)
	}
	var evidence interruptionEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.SchemaVersion != "gatec-interruption/v1" || evidence.Status != "lifecycle-timeout" || evidence.Attempt != 4 {
		t.Fatalf("unexpected interruption evidence: %+v", evidence)
	}
}

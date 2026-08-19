//go:build live

package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompleteFinalWorkerProcessComposesFileHandoff(t *testing.T) {
	if os.Getenv("ARDENTS_FINAL_HANDOFF_FIXTURE") == "1" {
		runFinalHandoffFixture(t)
		return
	}
	staging, secret := t.TempDir(), t.TempDir()
	t.Setenv("ARDENTS_BLOCKED_STAGING_ROOT", staging)
	root, err := prepareFinalWorkerRoot(strings.Repeat("e", 24))
	if err != nil {
		t.Fatal(err)
	}
	cell := "profile/C1/00"
	command := exec.CommandContext(context.Background(), os.Args[0], "-test.run", "^TestCompleteFinalWorkerProcessComposesFileHandoff$",
		"-test.count=1")
	command.Env = finalWorkerEnvironment(map[string]string{"ARDENTS_FINAL_HANDOFF_FIXTURE": "1",
		"ARDENTS_FINAL_WORKER_ROOT": root, "ARDENTS_FINAL_CELL": cell})
	results, _, err := completeFinalWorkerProcess(command, maximumFinalWorkerStream, time.Now(),
		cell, root, secret, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ObserverEvidence.Path == "" || results[0].TelemetryEvidence.Path == "" {
		t.Fatalf("composed handoff result=%+v", results)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("worker root remained after composed handoff: %v", err)
	}
	for _, artifact := range []finalRunnerArtifact{results[0].ObserverEvidence, results[0].TelemetryEvidence} {
		if _, err := os.Stat(filepath.Join(secret, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("published artifact %s: %v", artifact.Path, err)
		}
	}
}

func runFinalHandoffFixture(t *testing.T) {
	cell := os.Getenv("ARDENTS_FINAL_CELL")
	if err := writeFinalWorkerHandoff(os.Getenv("ARDENTS_FINAL_WORKER_ROOT"), cell,
		[]finalRawObserverSet{{}}, fixtureFinalRawTelemetry(cell, []byte("sample\n"))); err != nil {
		t.Fatal(err)
	}
	terminal := struct {
		Schema   string `json:"schema"`
		CellID   string `json:"cell_id"`
		Terminal string `json:"terminal"`
	}{Schema: "ardents-h3-final-worker-terminal-v1", CellID: cell, Terminal: "complete"}
	result := finalWorkerResult{Schema: "ardents-h3-final-worker-cell-v1", CellID: cell, Terminal: "complete"}
	for _, value := range []any{terminal, result} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(string(raw))
	}
}

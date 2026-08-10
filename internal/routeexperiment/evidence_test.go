package routeexperiment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalEvidenceBindsManifestAndMachineResult(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "input-manifest.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	summary := experimentSummary{SchemaVersion: experimentSchema, RunID: "run", Status: "completed", Decision: decisionAdvance, Conditions: map[string]conditionResult{}}
	if err := writeFinalEvidence(directory, summary); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"experiment.json", "experiment-verdict.json", "report.md"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || len(data) == 0 {
			t.Fatalf("final evidence %s is missing: %v", name, err)
		}
		if name == "report.md" && !strings.Contains(string(data), "Decision: **advance**") {
			t.Fatal("human report omits the decision")
		}
	}
}

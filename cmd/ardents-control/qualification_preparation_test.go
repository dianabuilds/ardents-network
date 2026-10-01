package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestQualificationPreparationRejectsInvalidPublicInputBeforeEffects(t *testing.T) {
	for _, operation := range []string{"prepare-qualification-evidence", "prepare-qualification-catalog", "prepare-qualification-node-record", "prepare-qualification-epoch"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			plan := filepath.Join(root, "plan.json")
			if err := os.WriteFile(plan, []byte(`{"schema":"unknown"}`), 0600); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(root, "absent")
			var receipt bytes.Buffer
			if err := run([]string{operation, "--plan", plan, "--output-root", output}, &receipt); err == nil {
				t.Fatal("invalid plan accepted")
			}
			if receipt.Len() != 0 {
				t.Fatal("refusal emitted success receipt")
			}
			if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatalf("pre-effect refusal changed output: %v", err)
			}
		})
	}
}

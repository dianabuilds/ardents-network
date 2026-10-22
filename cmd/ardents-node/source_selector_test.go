package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceConfigRetirementPrecedesPlanAndDiagnosticEffects(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(root, "diagnostics.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", socket)
	var output bytes.Buffer
	for _, plan := range []string{filepath.Join(root, "missing.json"), root} {
		err := runCommand(context.Background(), []string{"source", "--config", plan}, &output)
		if !errors.Is(err, errOldSourceProfileRetired) {
			t.Fatalf("retired selector reached a plan or diagnostic opener: %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired selector wrote output: %s", output.String())
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Fatalf("retired Source opened diagnostic socket: %v", err)
		}
	}
}

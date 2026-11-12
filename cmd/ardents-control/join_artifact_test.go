package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJoinPublicPreparationDoesNotAdmitRuntimeOrIncompleteInputs(t *testing.T) {
	for _, operation := range []string{"serve", "start-installed", "prepare-template", "prepare-descriptor", "prepare-inventory"} {
		t.Run(operation, func(t *testing.T) {
			parent := t.TempDir()
			outputPath := filepath.Join(parent, "output")
			var output bytes.Buffer
			err := run([]string{"join", operation, outputPath}, &output)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "join") || output.Len() != 0 {
				t.Fatal("unavailable JOIN operation published a result", err, output.String())
			}
			if _, err := os.Lstat(outputPath); !os.IsNotExist(err) {
				t.Fatal("invalid JOIN operation mutated output", err)
			}
		})
	}
}

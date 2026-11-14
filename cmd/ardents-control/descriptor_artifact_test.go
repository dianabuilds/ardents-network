package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDescriptorPublicPreparationDoesNotAdmitRuntimeOrIncompleteInputs(t *testing.T) {
	for _, operation := range []string{"serve", "start-installed", "prepare-template", "prepare-artifact", "prepare-inventory"} {
		t.Run(operation, func(t *testing.T) {
			parent := t.TempDir()
			outputPath := filepath.Join(parent, "output")
			var output bytes.Buffer
			args := []string{"descriptor", operation, outputPath}
			if operation == "prepare-template" {
				args = append(args, "unexpected")
			}
			err := run(args, &output)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "descriptor") || output.Len() != 0 {
				t.Fatal("unavailable Descriptor operation published a result", err, output.String())
			}
			if _, err := os.Lstat(outputPath); !os.IsNotExist(err) {
				t.Fatal("invalid Descriptor operation mutated output", err)
			}
		})
	}
}

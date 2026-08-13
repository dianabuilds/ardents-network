package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRejectsMissingAndMalformedEvidence(t *testing.T) {
	var output, diagnostics bytes.Buffer
	if code := run(nil, &output, &diagnostics); code != 2 {
		t.Fatalf("missing input exit=%d", code)
	}
	output.Reset()
	diagnostics.Reset()
	if code := run([]string{filepath.Join(t.TempDir(), "absent.json")}, &output, &diagnostics); code != 2 {
		t.Fatalf("absent input exit=%d", code)
	}
}

func TestRunRejectsTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, []byte(`{} {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output, diagnostics bytes.Buffer
	if code := run([]string{path}, &output, &diagnostics); code != 2 {
		t.Fatalf("trailing JSON exit=%d output=%s", code, output.String())
	}
}

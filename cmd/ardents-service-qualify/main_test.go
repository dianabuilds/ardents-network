package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCommandRendersInvalidEvidenceBeforeReturningFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(path, []byte(`{"schema":"wrong"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{path}, &output); err == nil || !bytes.Contains(output.Bytes(), []byte(`"verdict":"invalid"`)) {
		t.Fatalf("invalid evidence was not rendered: output=%s err=%v", output.Bytes(), err)
	}
}

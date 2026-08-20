//go:build !windows

package stage6verify_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func TestStage6VerifierRejectsSymlinkedArtifact(t *testing.T) {
	root := t.TempDir()
	writeEvidenceCampaign(t, root, "source-commit", "clean")
	path := filepath.Join(root, "evidence", "cells", "00", "terminal.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "terminal.json")
	if err = os.WriteFile(target, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
		filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
	if verdict.Status != "invalid" {
		t.Fatalf("verdict=%+v", verdict)
	}
}

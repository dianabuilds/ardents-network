package node_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/qualification/node"
)

func TestRunRejectsUnownedFixtureBeforeDocker(t *testing.T) {
	root := t.TempDir()
	result := node.Run(context.Background(), node.Campaign{
		FixtureRoot:  filepath.Join(root, "missing"),
		EvidenceRoot: filepath.Join(root, "evidence"),
		ComposeFile:  filepath.Join(root, "compose.yaml"),
		Mode:         "short",
	})
	if result.Verdict != "invalid" {
		t.Fatalf("verdict = %q, want invalid", result.Verdict)
	}
}

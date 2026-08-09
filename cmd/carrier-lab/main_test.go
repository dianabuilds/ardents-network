package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunRejectsMissingOrUnknownCommand(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{nil, {"unknown"}} {
		if status := run(arguments); status != 64 {
			t.Errorf("run(%q) = %d, want 64", arguments, status)
		}
	}
}

func TestCleanupFailureReturnsNonZeroAfterRemovingRunDirectory(t *testing.T) {
	t.Parallel()
	tempRoot := t.TempDir()
	runID := "20260809T120000Z-command"
	sessionRoot := filepath.Join(tempRoot, "ardents-carrier-lab-preflight-session."+runID)
	repositoryRoot := filepath.Join(tempRoot, "repository")
	runDir := filepath.Join(sessionRoot, "ardents-carrier-lab-preflight-run."+runID)
	evidenceDir := filepath.Join(sessionRoot, "ardents-carrier-lab-preflight-evidence."+runID)
	for _, directory := range []string{repositoryRoot, runDir, evidenceDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"schema_version":"carrier-lab-preflight-manifest/v1","run_id":"` + runID + `","status":"preflight_checks_passed"}`
	if err := os.WriteFile(filepath.Join(evidenceDir, "preflight-manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	status := run([]string{
		"finalize-cleanup",
		"--repository-root", repositoryRoot,
		"--session-root", sessionRoot,
		"--temp-root", tempRoot,
		"--run-id", runID,
		"--owned-containers-absent=true",
		"--owned-networks-absent=false",
		"--owned-volumes-absent=true",
	})
	if status == 0 {
		t.Fatal("cleanup failure returned a zero exit status")
	}
	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("cleanup failure left the run directory: %v", err)
	}
}

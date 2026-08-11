package namedsite

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTerminalBundleSupportsEarlyHardFailureRoleViews(t *testing.T) {
	directory := t.TempDir()
	startup := filepath.Join(t.TempDir(), "startup-evidence")
	if err := os.Mkdir(startup, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeBoundedJSON(filepath.Join(startup, "isolation.json"), map[string]any{"status": "failed"}); err != nil {
		t.Fatal(err)
	}
	if err := retainReferencePartialRoleViews(directory, 1, startup); err != nil {
		t.Fatal(err)
	}
	matrix := matrixResult{PositiveTotal: 20, Failures: make(map[string]bool), Verdict: "stop", Failure: "isolation failed"}
	if err := writeGateCBundle(directory, runManifest{RunID: "early-hard-failure"}, matrix, measurementSummary{}, true, time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"role-views.json", "receipt.json", "report.md", "result.json"} {
		if info, err := os.Stat(filepath.Join(directory, name)); err != nil || info.Size() == 0 {
			t.Fatalf("terminal evidence %s is missing: %v", name, err)
		}
	}
}

func TestReferenceOnlyCleanupIsComplete(t *testing.T) {
	directory := t.TempDir()
	attempt := filepath.Join(directory, "attempts", "001")
	if err := os.MkdirAll(attempt, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeReferenceOnlyAttemptCleanup(directory, 1); err != nil {
		t.Fatal(err)
	}
	if !attemptCleanupProven(directory, 1) {
		t.Fatal("reference-only cleanup was not accepted for a Route that never started")
	}
}

func TestNativeAttachedOutcomeRequiresScenarioAndCleanupEvidence(t *testing.T) {
	directory := t.TempDir()
	write := func(kind string, cleanup bool) {
		t.Helper()
		if err := writeBoundedJSON(filepath.Join(directory, "native-run.json"), map[string]any{
			"schema_version": "carrier-lab-native-run/v1", "status": "failed", "failure_kind": kind,
			"checks": map[string]bool{"cleanup_complete": cleanup},
		}); err != nil {
			t.Fatal(err)
		}
	}
	write("scenario", true)
	scenario, err := nativeAttachedScenarioFailed(directory)
	if err != nil || !scenario {
		t.Fatalf("scenario outcome was not recognized: scenario=%t err=%v", scenario, err)
	}
	write("operational", true)
	scenario, err = nativeAttachedScenarioFailed(directory)
	if err != nil || scenario {
		t.Fatalf("operational outcome was misclassified: scenario=%t err=%v", scenario, err)
	}
	write("scenario", false)
	if _, err := nativeAttachedScenarioFailed(directory); err == nil {
		t.Fatal("scenario outcome without cleanup proof was accepted")
	}
}

func TestPartialRoleViewsPlaceholderOnlyMissingFiles(t *testing.T) {
	retained := t.TempDir()
	source := t.TempDir()
	if err := retainReferencePartialRoleViews(retained, 1, source); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"isolation.json", "relay/relay.json", "gateway/gateway.json"} {
		if _, err := os.Stat(filepath.Join(retained, "attempts", "001", "reference", filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing role-view placeholder %s: %v", relative, err)
		}
	}
	invalid := filepath.Join(source, "relay", "relay.json")
	if err := os.MkdirAll(filepath.Dir(invalid), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := retainReferencePartialRoleViews(t.TempDir(), 1, source); err == nil {
		t.Fatal("invalid role-view evidence was replaced by a placeholder")
	}
}

func TestRetainNativeRunEvidenceSupportsTerminalReferenceFailure(t *testing.T) {
	directory := t.TempDir()
	if err := writeBoundedJSON(filepath.Join(directory, "native-run.json"), map[string]any{
		"schema_version": "carrier-lab-native-run/v1", "status": "passed", "checks": map[string]bool{"cleanup_complete": true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := retainNativeRunEvidence(directory, 1); err != nil {
		t.Fatal(err)
	}
	if err := writeAttemptCleanup(directory, 1); err != nil {
		t.Fatal(err)
	}
	if !attemptCleanupProven(directory, 1) {
		t.Fatal("successful native cleanup receipt was not retained for a terminal reference failure")
	}
}

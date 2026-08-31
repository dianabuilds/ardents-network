//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestH48A11FailedAttemptCaptureIsRetainedBeforeDestructiveCleanup(t *testing.T) {
	root := t.TempDir()
	status := h48A11Status{root: root}
	order := make([]string, 0, 2)
	if err := status.retainRemoteFailureWith(func() ([]byte, error) {
		order = append(order, "capture")
		return []byte("schema=ardents-h4-8-a11-remote-evidence-v1\n[role-output]\n"), nil
	}); err != nil {
		t.Fatal(err)
	}
	order = append(order, "remove")
	if strings.Join(order, ",") != "capture,remove" {
		t.Fatalf("failure cleanup order = %v", order)
	}
	for _, name := range []string{"remote-evidence/capture.txt", "remote-evidence.failed-attempt"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || len(raw) == 0 {
			t.Fatalf("failed-attempt evidence %s = %q / %v", name, raw, err)
		}
	}
}

func TestH48A11FailedAttemptCaptureDoesNotDuplicateSuccessfulEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "remote-evidence.complete"), []byte("retained\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	status := h48A11Status{root: root}
	if err := status.retainRemoteFailureWith(func() ([]byte, error) {
		called = true
		return []byte("unexpected"), nil
	}); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("successful remote evidence was captured a second time")
	}
}

func TestH48A11FailureCaptureCommandToleratesMissingContainerAndStage(t *testing.T) {
	command := h48A11RemoteEvidenceCommand(h43MultiHostEnvironment{container: "attempt", remoteDirectory: "/tmp/attempt"})
	for _, required := range []string{`"container_available":false`, "staged_root_available=false", "$(docker inspect", "if [ -d"} {
		if !strings.Contains(command, required) {
			t.Fatalf("failure capture command is missing %q", required)
		}
	}
	if strings.Contains(command, "set -e") {
		t.Fatal("failure capture aborts before absent-state inventory")
	}
}

func TestH48A11SuccessCaptureKeepsExactV1ParserGrammar(t *testing.T) {
	command := h48A11RemoteEvidenceCommand(h43MultiHostEnvironment{container: "attempt", remoteDirectory: "/tmp/attempt"})
	if strings.Contains(command, "container_available=true") || strings.Contains(command, "staged_root_available=true") {
		t.Fatal("successful capture added non-JSON or non-digest availability rows")
	}
	for _, required := range []string{
		`printf '%s\n' "$container_state"`,
		`printf '[staged-inventory-sha256]\n'`,
		`if [ -f "$path" ]`,
		`sha256sum "$path" 2>/dev/null`,
		`printf '%s\t%s\t%s\n' "$digest" "$bytes" "${path#./}"`,
		`printf '[role-exit-statuses]\n'`,
		`cat remote-role-exit-statuses.jsonl`,
		`tail -c 16384 "$file"`,
		`printf '[role-output]\n'`,
	} {
		if !strings.Contains(command, required) {
			t.Fatalf("successful capture v1 grammar is missing %q", required)
		}
	}
}

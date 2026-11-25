//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// StartOriginalEndpoint starts the staged product and returns its observed invocation.
func StartOriginalEndpoint(t testing.TB) [3]string {
	t.Helper()
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("platform-led start: %v: %s", err, out)
	}
	original := StartupIdentity(t, t.Context())
	if original[0] == "0" || len(original[1]) != 32 || original[2] != "active" {
		t.Fatal("platform-led participant is not active", original)
	}
	return original
}

// StopOriginalEndpoint stops the matching original invocation.
func StopOriginalEndpoint(t testing.TB, original [3]string) {
	t.Helper()
	if current := StartupIdentity(t, t.Context()); current != original {
		t.Errorf("original installed invocation changed: %v", current)
		return
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Errorf("original stop: %v: %s", err, out)
	}
}

// RequireJoinedEndpoint checks the manager's result before reload or reset discards the
// original exit observation. Stop job success alone does not establish it.
func RequireJoinedEndpoint(t testing.TB, original [3]string) {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-endpoint.service",
		"--all", "--property=ActiveState,SubState,MainPID,ExecMainPID,ExecMainCode,ExecMainStatus,Result,Job").Output()
	if err != nil {
		t.Fatal("observe stopped Endpoint:", err)
	}
	properties := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid stopped Endpoint property %q", line)
		}
		properties[name] = value
	}
	for name, want := range map[string]string{
		"ActiveState": "inactive", "SubState": "dead", "MainPID": "0", "ExecMainPID": original[0],
		"ExecMainCode": "1", "ExecMainStatus": "0", "Result": "success", "Job": "",
	} {
		got, ok := properties[name]
		if !ok || got != want {
			t.Fatalf("stopped Endpoint %s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join("/proc", original[0])); !os.IsNotExist(err) {
		t.Fatalf("original Endpoint PID %s still present or unobservable: %v", original[0], err)
	}
}

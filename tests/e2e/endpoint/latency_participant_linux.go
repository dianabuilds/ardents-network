//go:build linux && endpoint_launch_native

package endpoint

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// InstallLatencyParticipant builds the endpoint-package qualification test binary
// (the non-root MainPID) and installs it root-owned immutable, mirroring the
// installed text-worker qualification pattern. A prebuilt binary may be supplied
// to avoid a Go build under shared-host load; its bytes are still pinned.
func InstallLatencyParticipant(t *testing.T) string {
	t.Helper()
	const installed = "/usr/lib/ardents/qualification/latency-participant.test"
	var raw []byte
	if prebuilt := os.Getenv("ARDENTS_LATENCY_PREBUILT_PARTICIPANT"); prebuilt != "" {
		data, err := os.ReadFile(prebuilt)
		if err != nil {
			t.Fatal(err)
		}
		raw = data
	} else {
		repoRoot := os.Getenv("ARDENTS_LATENCY_REPO_ROOT")
		if repoRoot == "" {
			repoRoot = RepositoryRoot(t)
		}
		out := filepath.Join(t.TempDir(), "latency-participant.test")
		cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-tags=endpoint_launch_native", "-trimpath", "-buildvcs=false", "-o", out, "./internal/domain/endpoint")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build cold/warm participant: %v: %s", err, output)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		raw = data
	}
	if err := os.MkdirAll(filepath.Dir(installed), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, raw, 0555); err != nil {
		t.Fatal(err)
	}
	return installed
}

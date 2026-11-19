//go:build linux

package endpoint_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The retired ACA1 inspection and qualification-preparation routes refuse
// against the exact headless control artifact before parsing flags or opening
// any file, root, or floor (ADR-0127, #647). This keeps the headless-evidence
// control-artifact binding exercised after the reader's retirement; the
// accepting closed-profile and infrastructure preparation routes retain their
// own command and native evidence.
func TestRetiredControlRoutesRefuseBeforeEffectsAgainstExactArtifact(t *testing.T) {
	control := buildControl(t)
	directory := t.TempDir()
	for _, route := range []struct{ name, refusal string }{
		{"inspect-bundle", "inspect-bundle is retired"},
		{"inspect-transitions", "inspect-transitions is retired"},
		{"prepare-qualification-evidence", "prepare-qualification-evidence is retired"},
		{"prepare-qualification-catalog", "prepare-qualification-catalog is retired"},
		{"prepare-qualification-node-record", "prepare-qualification-node-record is retired; use ardents-next network prepare-initial-node-record"},
		{"prepare-qualification-epoch", "prepare-qualification-epoch is retired; use ardents-next network prepare-initial-epoch"},
	} {
		t.Run(route.name, func(t *testing.T) {
			output, err := exec.Command(control, route.name,
				"--plan", filepath.Join(directory, "absent-plan.json"),
				"--output-root", filepath.Join(directory, "absent-output")).CombinedOutput()
			if err == nil || !strings.Contains(string(output), route.refusal) {
				t.Fatalf("retired %s = %v\n%s", route.name, err, output)
			}
		})
	}
	if _, err := os.Lstat(filepath.Join(directory, "absent-output")); !os.IsNotExist(err) {
		t.Fatalf("retired control routes created an output root: %v", err)
	}
}

func buildControl(t *testing.T) string {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_E2E_CONTROL"); prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt alpha-control command is not a regular file: %v", err)
		}
		return prebuilt
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E source")
	}
	path := filepath.Join(t.TempDir(), "ardents-control")
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", path, "./cmd/ardents-control")
	build.Dir = filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build control reader: %v\n%s", err, output)
	}
	return path
}

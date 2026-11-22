//go:build linux

package endpoint_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The retired ACA1 inspection and qualification-preparation routes refuse
// against the exact headless Control artifact before parsing flags or opening
// any file, root, or floor (ADR-0127, #647). This keeps the headless-evidence
// control-artifact binding exercised after reader retirement. The same
// artifact exercises the removed transitional infrastructure preparation.
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

	t.Run("removed-infrastructure-preparation", func(t *testing.T) {
		probe := filepath.Join(directory, "input")
		original := []byte("retained input")
		if err := os.WriteFile(probe, original, 0600); err != nil {
			t.Fatal(err)
		}
		for _, role := range []struct {
			name, diagnostic string
			operations       []string
		}{
			{"source", "usage: ardents-control source OPERATION [ARGUMENTS]", []string{"prepare-template", "prepare-install-inputs"}},
			{"forwarding", "forwarding artifact preparation refused", []string{"prepare-template", "prepare-request", "prepare-plan", "prepare-hosting", "prepare-state"}},
			{"join", "JOIN artifact preparation refused", []string{"prepare-template", "prepare-request", "prepare-plan", "prepare-hosting", "prepare-state"}},
			{"descriptor", "Descriptor artifact preparation refused", []string{"prepare-template", "prepare-request", "prepare-plan", "prepare-hosting", "prepare-state"}},
		} {
			for _, operation := range role.operations {
				t.Run(role.name+"/"+operation, func(t *testing.T) {
					destination := filepath.Join(t.TempDir(), "output")
					command := exec.CommandContext(t.Context(), control, role.name, operation, probe, destination)
					var output, diagnostic bytes.Buffer
					command.Stdout, command.Stderr = &output, &diagnostic
					var exit *exec.ExitError
					err := command.Run()
					if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 ||
						diagnostic.String() != role.diagnostic+"\n" {
						t.Fatalf("removed operation returned %v, stdout %q, stderr %q", err, output.String(), diagnostic.String())
					}
					if _, err := os.Lstat(destination); !os.IsNotExist(err) {
						t.Fatal("removed operation created output", err)
					}
					actual, err := os.ReadFile(probe)
					if err != nil || !bytes.Equal(actual, original) {
						t.Fatal("removed operation changed the original input", err)
					}
				})
			}
		}
	})
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

//go:build !linux

package source_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompiledSourceUnsupportedPlatformRefusesBeforePlanEffects(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	binary := buildSourceCommand(t)
	for _, route := range []struct {
		command, operation string
		bundle, stdout     bool
	}{
		{"serve", "source", false, false},
		{"prepare-state", "source.prepare-state", true, false},
		{"prepare-trust", "source.prepare-trust", true, true},
	} {
		t.Run(route.command, func(t *testing.T) {
			root := t.TempDir()
			args := []string{"source", route.command, filepath.Join(root, "absent-plan")}
			if route.bundle {
				args = append(args, filepath.Join(root, "absent-bundle"))
			}
			command := exec.CommandContext(t.Context(), binary, args...)
			var out, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &out, &diagnostic
			var exit *exec.ExitError
			if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatalf("unsupported Source returned %v, stdout %q, stderr %q", err, &out, &diagnostic)
			}
			report, silent := &diagnostic, &out
			if route.stdout {
				report, silent = &out, &diagnostic
			}
			var event map[string]string
			if json.Unmarshal(report.Bytes(), &event) != nil || len(event) != 3 ||
				event["operation"] != route.operation || event["phase"] != "refused" || event["outcome"] != "unsupported-platform" || silent.Len() != 0 {
				t.Fatalf("unexpected refusal for %s: stdout %q, stderr %q", route.operation, &out, &diagnostic)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("unsupported Source modified the original directory", entries, err)
			}
		})
	}
}

package ceremony_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalCeremonyArtifactsRejectUntrustedArgumentsBeforeSecretInput(t *testing.T) {
	release := requiredArtifact(t, "ARDENTS_E2E_RELEASE_CUSTODY")
	state := requiredArtifact(t, "ARDENTS_E2E_STATE_CUSTODY")
	tests := []struct {
		name      string
		artifact  string
		arguments []string
		root      string
		wantError string
	}{
		{name: "release initialize", artifact: release, arguments: []string{"initialize", "--root", "relative-release-initialize"}, root: "relative-release-initialize", wantError: "release custody arguments are invalid"},
		{name: "release inspect", artifact: release, arguments: []string{"inspect", "--root", "relative-release-inspect"}, root: "relative-release-inspect", wantError: "release custody arguments are invalid"},
		{name: "State initialize", artifact: state, arguments: []string{"initialize-alpha-genesis", "--root", "relative-state-initialize"}, root: "relative-state-initialize", wantError: "functional alpha State custody arguments are invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			working := t.TempDir()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, test.artifact, test.arguments...)
			command.Dir = working
			output, err := command.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("local ceremony route did not terminate within its bound: %v", ctx.Err())
			}
			if err == nil {
				t.Fatalf("local ceremony route accepted an untrusted relative root: %s", output)
			}
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 2 {
				t.Fatalf("local ceremony route exit = %v, output=%s", err, output)
			}
			if got := strings.TrimSpace(string(output)); got != test.wantError {
				t.Fatalf("local ceremony route error = %q, want pre-secret argument rejection %q", got, test.wantError)
			}
			if _, statErr := os.Stat(filepath.Join(working, test.root)); !os.IsNotExist(statErr) {
				t.Fatalf("rejected local ceremony route created state: %v", statErr)
			}
		})
	}
}

func requiredArtifact(t *testing.T, name string) string {
	t.Helper()
	path := os.Getenv(name)
	if !filepath.IsAbs(path) {
		t.Fatalf("%s must name the exact built local ceremony artifact", name)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("%s artifact is unavailable: %v", name, err)
	}
	return path
}

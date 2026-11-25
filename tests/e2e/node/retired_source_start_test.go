package node_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPlatformRetiresDetachedSourceStartBeforeEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "infrastructure", "start-source", root, root, "closed", "private", root)
	var output bytes.Buffer
	command.Stdout = &output
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 {
		t.Fatalf("retired Source startup: %v, output %q", err, &output)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("retired Source startup touched input: %v", err)
	}
}

func TestInfrastructureCompiledRejectsRetiredSourceOperations(t *testing.T) {
	program := buildNodeCommand(t)
	for _, operation := range []string{"preflight-initial", "preflight-candidate", "install-initial-stopped"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "absent")
			cmd := exec.CommandContext(t.Context(), program, "infrastructure", operation, path)
			out, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 || len(out) != 0 {
				t.Fatalf("retired operation returned %q, %v", out, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("retired operation changed input path", err)
			}
		})
	}
}

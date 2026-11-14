//go:build linux || windows

package service_test

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

func runCommand(t *testing.T, ctx context.Context, root, binary string, arguments ...string) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, output)
	}
	return output
}

func runInteractiveProductCommand(t *testing.T, directory, binary string, inputs []interactiveProductInput, arguments ...string) []byte {
	t.Helper()
	output, err := runInteractiveProductCommandResult(t, directory, binary, inputs, arguments...)
	if err != nil {
		t.Fatalf("interactive product command failed: %v\n%s", err, output)
	}
	return output
}

func interactiveProductJSON(t *testing.T, terminal []byte) []byte {
	t.Helper()
	start, end := bytes.IndexByte(terminal, '{'), bytes.LastIndexByte(terminal, '}')
	if start < 0 || end < start {
		t.Fatalf("interactive product output lacks one JSON receipt: %q", terminal)
	}
	return append([]byte(nil), terminal[start:end+1]...)
}

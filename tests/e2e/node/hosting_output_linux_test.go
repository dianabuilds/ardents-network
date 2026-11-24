//go:build linux

package node_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"
)

func TestHostingReportsOutputFailureAfterInitialization(t *testing.T) {
	plan := hostingInitializePlan(t)
	config := admissionCycleConfig(t, map[string]any{"root": plan.Root, "policy": plan.Policy})
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	command := exec.CommandContext(t.Context(), buildNodeCommand(t), "hosting", "initialize", "--config", config)
	var diagnostic bytes.Buffer
	command.Stdout, command.Stderr = full, &diagnostic
	err = command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("output failure: %v, stderr %q", err, &diagnostic)
	}
	if _, err := os.Stat(plan.Root); err != nil {
		t.Fatalf("initialized budget unavailable: %v", err)
	}
}

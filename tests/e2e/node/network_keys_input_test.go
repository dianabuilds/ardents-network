package node_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNetworkInitialKeysRefusesRelativePathBeforeEffects(t *testing.T) {
	binary := buildNodeCommand(t)
	destination := filepath.Join(t.TempDir(), "destination")
	command := exec.CommandContext(t.Context(), binary, "network", "prepare-initial-keys", "relative", destination)
	var output, diagnostic bytes.Buffer
	command.Stdout, command.Stderr = &output, &diagnostic
	err := command.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 || output.Len() != 0 {
		t.Fatalf("invalid paths: %v, stdout %q, stderr %q", err, &output, &diagnostic)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("invalid path touched destination: %v", err)
	}
}

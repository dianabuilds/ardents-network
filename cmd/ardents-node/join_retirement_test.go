package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestJoinOrchestrationRetirementPrecedesEffects(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(root, "diagnostics.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", socket)
	for _, test := range []struct {
		arguments []string
		retired   error
	}{
		{[]string{"join", "start-installed", filepath.Join(root, "binding.json")}, errOldJoinInstalledRetired},
		{[]string{"join", "install-initial-stopped", "dist", "pin", "history", "env", "network", "request"}, errOldJoinInstallationRetired},
		{[]string{"join", "stop", filepath.Join(root, "history"), filepath.Join(root, "binding.json")}, errOldJoinInstallationRetired},
		{[]string{"join", "supervise-transition", filepath.Join(root, "request.json")}, errOldJoinInstallationRetired},
		{[]string{"join", "inspect-inputs", filepath.Join(root, "request.json")}, errOldJoinInstallationRetired},
		{[]string{"join"}, errOldJoinInstallationRetired},
	} {
		var output bytes.Buffer
		err := runCommand(context.Background(), test.arguments, &output)
		if !errors.Is(err, test.retired) {
			t.Fatalf("retired JOIN orchestration reached an effect: %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired JOIN orchestration wrote output: %s", output.String())
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Fatalf("retired JOIN orchestration opened diagnostic socket: %v", err)
		}
	}
}

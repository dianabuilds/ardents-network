package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestForwardingOrchestrationRetirementPrecedesEffects(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(root, "diagnostics.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", socket)
	for _, test := range []struct {
		arguments []string
		retired   error
	}{
		{[]string{"forwarding", "start-installed", filepath.Join(root, "binding.json")}, errOldForwardingInstalledRetired},
		{[]string{"forwarding", "install-initial-stopped", "dist", "pin", "history", "env", "network", "request"}, errOldForwardingInstallationRetired},
		{[]string{"forwarding", "stop", filepath.Join(root, "history"), filepath.Join(root, "binding.json")}, errOldForwardingInstallationRetired},
		{[]string{"forwarding", "supervise-transition", filepath.Join(root, "request.json")}, errOldForwardingInstallationRetired},
		{[]string{"forwarding", "inspect-inputs", filepath.Join(root, "request.json")}, errOldForwardingInstallationRetired},
		{[]string{"forwarding"}, errOldForwardingInstallationRetired},
	} {
		var output bytes.Buffer
		err := runCommand(context.Background(), test.arguments, &output)
		if !errors.Is(err, test.retired) {
			t.Fatalf("retired Forwarding orchestration reached an effect: %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired Forwarding orchestration wrote output: %s", output.String())
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Fatalf("retired Forwarding orchestration opened diagnostic socket: %v", err)
		}
	}
}

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptorOrchestrationRetirementPrecedesEffects(t *testing.T) {
	root := t.TempDir()
	socket := filepath.Join(root, "diagnostics.socket")
	t.Setenv("ARDENTS_DEBUG_SOCKET", socket)
	for _, test := range []struct {
		arguments []string
		retired   error
	}{
		{[]string{"descriptor", "start-installed", filepath.Join(root, "binding.json")}, errOldDescriptorInstalledRetired},
		{[]string{"descriptor", "install-initial-stopped", "dist", "pin", "history", "env", "network", "request"}, errOldDescriptorInstallationRetired},
		{[]string{"descriptor", "stop", filepath.Join(root, "history"), filepath.Join(root, "binding.json")}, errOldDescriptorInstallationRetired},
		{[]string{"descriptor", "supervise-transition", filepath.Join(root, "request.json")}, errOldDescriptorInstallationRetired},
		{[]string{"descriptor", "inspect-inputs", filepath.Join(root, "request.json")}, errOldDescriptorInstallationRetired},
		{[]string{"descriptor"}, errOldDescriptorInstallationRetired},
	} {
		var output bytes.Buffer
		err := runCommand(context.Background(), test.arguments, &output)
		if !errors.Is(err, test.retired) {
			t.Fatalf("retired Descriptor orchestration reached an effect: %v", err)
		}
		if output.Len() != 0 {
			t.Fatalf("retired Descriptor orchestration wrote output: %s", output.String())
		}
		if _, err := os.Lstat(socket); !os.IsNotExist(err) {
			t.Fatalf("retired Descriptor orchestration opened diagnostic socket: %v", err)
		}
	}
}

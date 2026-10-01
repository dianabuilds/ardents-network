//go:build !linux

package state_test

import (
	"os"
	"os/exec"
	"testing"
)

func nodeProcessCommand(t *testing.T, binary string, arguments ...string) (*exec.Cmd, func(*testing.T)) {
	t.Helper()
	if os.Getenv("ARDENTS_NODE_SCOPE_PROFILE") != "" {
		t.Fatal("invalid prerequisite: Node system scopes require Linux")
	}
	return exec.Command(binary, arguments...), nil
}

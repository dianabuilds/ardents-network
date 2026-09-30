//go:build !linux || !text_worker_installed

package state_test

import (
	"os/exec"
	"testing"
)

func nodeProcessCommand(t *testing.T, binary string, arguments ...string) (*exec.Cmd, func(*testing.T)) {
	t.Helper()
	return exec.Command(binary, arguments...), nil
}

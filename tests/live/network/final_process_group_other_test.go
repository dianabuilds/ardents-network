//go:build live && !linux && !windows

package network_test

import (
	"os"
	"os/exec"
)

func prepareFinalProcess(_ *exec.Cmd) {}

func terminateFinalProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}

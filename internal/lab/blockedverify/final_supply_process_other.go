//go:build !linux && !windows

package blockedverify

import (
	"os"
	"os/exec"
)

func prepareSupplyProcess(_ *exec.Cmd) {}

func terminateSupplyProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}

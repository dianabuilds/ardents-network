//go:build !linux && !windows

package blockedentry

import (
	"os"
	"os/exec"
)

func prepareReceiptProcess(_ *exec.Cmd) {}

func terminateReceiptProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}

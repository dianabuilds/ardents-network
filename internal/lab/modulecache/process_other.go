//go:build !linux && !windows

package modulecache

import (
	"errors"
	"os"
	"os/exec"
)

func prepareModuleProcess(_ *exec.Cmd) error {
	return errors.New("bounded module-cache generation is supported only on Linux and Windows")
}

func terminateModuleProcess(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}

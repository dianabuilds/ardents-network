package worker

import (
	"os/exec"
	"runtime"
	"syscall"
)

// Retain the creating thread through actual helper join. Fatal Endpoint death
// must not leave an original helper to submit a late manager operation.
func runOriginalCommand(command *exec.Cmd) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return command.Run()
}

package processfixture

import (
	"errors"
	"os/exec"
	"syscall"
)

// PrepareInterrupt starts the child in its own process group. It must inherit
// the test runner's console so an addressed CTRL_BREAK reaches that group.
func PrepareInterrupt(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// Interrupt requests graceful shutdown without signaling the test runner.
func Interrupt(command *exec.Cmd) error {
	if command == nil || command.Process == nil || command.Process.Pid <= 0 {
		return errors.New("interrupt process unavailable")
	}
	generate := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
	const ctrlBreakEvent = 1
	ok, _, err := generate.Call(ctrlBreakEvent, uintptr(command.Process.Pid))
	if ok == 0 {
		if err == syscall.Errno(0) {
			return errors.New("console interruption failed")
		}
		return err
	}
	return nil
}

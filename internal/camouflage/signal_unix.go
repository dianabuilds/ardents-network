//go:build !windows

package camouflage

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureCandidateProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalTerminate(process *os.Process) error { return syscall.Kill(-process.Pid, syscall.SIGTERM) }

func signalKill(process *os.Process) error { return syscall.Kill(-process.Pid, syscall.SIGKILL) }

func cleanupProcessGroup(pid int, deadline time.Time) error {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-pid, 0); errors.Is(err, syscall.ESRCH) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("candidate process group residue remains")
}

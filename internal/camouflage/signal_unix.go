//go:build !windows

package camouflage

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const candidateUID = 65532

func configureCandidateProcess(command *exec.Cmd, stateRoot string) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if os.Geteuid() != 0 {
		return nil
	}
	if stateRoot == "" {
		return errors.New("candidate state root is absent")
	}
	if err := os.Chown(stateRoot, candidateUID, candidateUID); err != nil {
		return err
	}
	command.SysProcAttr.Credential = &syscall.Credential{
		Uid: candidateUID, Gid: candidateUID, NoSetGroups: true,
	}
	return nil
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

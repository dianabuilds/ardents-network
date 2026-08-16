//go:build windows

package camouflage

import (
	"os"
	"os/exec"
	"time"
)

func configureCandidateProcess(*exec.Cmd, string) error { return nil }

func signalTerminate(process *os.Process) error { return process.Signal(os.Interrupt) }

func signalKill(process *os.Process) error { return process.Kill() }

func cleanupProcessGroup(int, time.Time) error { return nil }

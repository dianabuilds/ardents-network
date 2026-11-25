//go:build !windows

package processfixture

import (
	"os"
	"os/exec"
)

// PrepareInterrupt configures a process for its host's graceful interruption.
func PrepareInterrupt(_ *exec.Cmd) {}

// Interrupt requests graceful shutdown of the original process.
func Interrupt(command *exec.Cmd) error { return command.Process.Signal(os.Interrupt) }

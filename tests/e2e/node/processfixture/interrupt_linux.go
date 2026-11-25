package processfixture

import (
	"os"
	"os/exec"
	"testing"
)

// Interrupt requests graceful shutdown of the original process.
func Interrupt(command *exec.Cmd) error { return command.Process.Signal(os.Interrupt) }

// OpenAdmissionConsoleWithInterrupt exposes graceful Linux process interruption and join.
func OpenAdmissionConsoleWithInterrupt(t testing.TB, binary, operation, config string) (func(any) AdmissionReply, func() error, func() error) {
	return openAdmissionConsole(t, binary, operation, config, Interrupt)
}

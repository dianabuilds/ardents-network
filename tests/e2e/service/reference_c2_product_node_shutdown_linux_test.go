//go:build linux

package service_test

import (
	"os"
	"syscall"
)

// referenceC2RequestProductNodeShutdown uses the same normal termination
// request Docker and a Linux service manager issue to the product process.
func referenceC2RequestProductNodeShutdown(process *os.Process) (bool, error) {
	return true, process.Signal(syscall.SIGTERM)
}

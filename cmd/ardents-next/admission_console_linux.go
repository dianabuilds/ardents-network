//go:build linux

package main

import (
	"errors"
	"os"
	"syscall"
)

// NewFile notices O_NONBLOCK and attaches inherited pipes to Go's poller.
// Closing the owned duplicate then interrupts Read/Write instead of leaving a
// thread trapped in a blocking syscall on the original inherited descriptor.
func admissionConsoleFile(file *os.File) (*os.File, error) {
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(fd)
	if err = syscall.SetNonblock(fd, true); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	owned := os.NewFile(uintptr(fd), "admission-console")
	if owned == nil {
		_ = syscall.Close(fd)
		return nil, errors.New("console unavailable")
	}
	return owned, nil
}

//go:build unix

package admission

import (
	"errors"
	"os"
	"syscall"
)

// OpenConsoleFile returns an owned duplicate that interruption can close.
// NewFile notices O_NONBLOCK and attaches inherited pipes to Go's poller.
// Closing the owned duplicate then interrupts Read/Write instead of leaving a
// thread trapped in a blocking syscall on the original inherited descriptor.
func OpenConsoleFile(file *os.File) (*os.File, error) {
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(fd)
	if err = syscall.SetNonblock(fd, true); err != nil {
		return nil, errors.Join(err, syscall.Close(fd))
	}
	owned := os.NewFile(uintptr(fd), "admission-console")
	if owned == nil {
		return nil, errors.Join(errors.New("console unavailable"), syscall.Close(fd))
	}
	return owned, nil
}

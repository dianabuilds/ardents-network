//go:build linux

package main

import (
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

// A journal socket cannot be reopened through procfs. Keep a duplicate without
// changing its shared file status flags, and use per-call nonblocking sends.
// Deadline changes wake the bounded retry wait; Close joins every syscall
// under the same lock before releasing the descriptor.
type headlessTextSocketOutput struct {
	mu       sync.Mutex
	fd       int
	closed   bool
	deadline time.Time
	changed  chan struct{}
}

func openHeadlessTextSocket(inherited *os.File) (*headlessTextSocketOutput, error) {
	raw, err := inherited.SyscallConn()
	if err != nil {
		return nil, err
	}
	descriptor := -1
	var duplicateErr error
	if err := raw.Control(func(fd uintptr) {
		duplicated, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_DUPFD_CLOEXEC, 0)
		if errno != 0 {
			duplicateErr = errno
		} else {
			descriptor = int(duplicated)
		}
	}); err != nil {
		return nil, err
	}
	if duplicateErr != nil {
		return nil, duplicateErr
	}
	kind, err := syscall.GetsockoptInt(descriptor, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil || kind != syscall.SOCK_STREAM {
		return nil, errors.Join(errors.New("text runtime requires stream event socket"), err, syscall.Close(descriptor))
	}
	return &headlessTextSocketOutput{fd: descriptor, changed: make(chan struct{})}, nil
}

func (output *headlessTextSocketOutput) SetWriteDeadline(deadline time.Time) error {
	output.mu.Lock()
	defer output.mu.Unlock()
	if output.closed {
		return os.ErrClosed
	}
	output.deadline = deadline
	close(output.changed)
	output.changed = make(chan struct{})
	return nil
}

func (output *headlessTextSocketOutput) Write(value []byte) (int, error) {
	total := 0
	for total < len(value) {
		output.mu.Lock()
		if output.closed {
			output.mu.Unlock()
			return total, os.ErrClosed
		}
		deadline := output.deadline
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			output.mu.Unlock()
			return total, os.ErrDeadlineExceeded
		}
		n, err := syscall.SendmsgN(output.fd, value[total:], nil, nil, syscall.MSG_DONTWAIT|syscall.MSG_NOSIGNAL)
		changed := output.changed
		output.mu.Unlock()
		if n > 0 {
			total += n
		}
		if err == nil {
			if n == 0 {
				return total, io.ErrNoProgress
			}
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EINTR) {
			return total, err
		}
		delay := 20 * time.Millisecond
		if !deadline.IsZero() && time.Until(deadline) < delay {
			delay = time.Until(deadline)
		}
		timer := time.NewTimer(delay)
		select {
		case <-changed:
		case <-timer.C:
		}
		timer.Stop()
	}
	return total, nil
}

func (output *headlessTextSocketOutput) Close() error {
	output.mu.Lock()
	defer output.mu.Unlock()
	if output.closed {
		return nil
	}
	output.closed = true
	close(output.changed)
	return syscall.Close(output.fd)
}

//go:build linux || darwin

package hosting

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

type hostingLease struct{ file *os.File }

func acquireHostingLease(ctx context.Context, root *os.Root) (*hostingLease, error) {
	return acquireHostingLeaseMode(ctx, root, syscall.LOCK_EX)
}

func acquireHostingLeaseMode(ctx context.Context, root *os.Root, mode int) (*hostingLease, error) {
	file, err := openHostingLease(root)
	if err != nil {
		return nil, err
	}
	wait, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	retryDelay := time.Millisecond
	for {
		if err := wait.Err(); err != nil {
			return nil, errors.Join(err, file.Close())
		}
		err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB)
		if err == nil {
			return &hostingLease{file: file}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return nil, errors.Join(err, file.Close())
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-wait.Done():
			timer.Stop()
		case <-timer.C:
		}
		if retryDelay < 32*time.Millisecond {
			retryDelay *= 2
		}
	}
}

func openHostingLease(root *os.Root) (*os.File, error) {
	before, err := root.Lstat(periodLeaseFile)
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("hosting lock is unavailable")
	}
	file, err := root.OpenFile(periodLeaseFile, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(errors.New("hosting lock changed"), file.Close())
	}
	return file, nil
}

func (lease *hostingLease) close() error {
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}

//go:build linux

package resource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type hostingLease struct{ file *os.File }

func hostingPlatform() error { return nil }

func acquireHostingLease(ctx context.Context, root *os.Root) (*hostingLease, error) {
	return acquireHostingLeaseMode(ctx, root, syscall.LOCK_EX)
}

func acquireHostingReadLease(ctx context.Context, root *os.Root) (*hostingLease, error) {
	return acquireHostingLeaseMode(ctx, root, syscall.LOCK_SH)
}

func acquireHostingLeaseMode(ctx context.Context, root *os.Root, mode int) (*hostingLease, error) {
	before, err := root.Lstat("period.lock")
	if err != nil || !before.Mode().IsRegular() {
		return nil, errors.New("hosting lock is unavailable")
	}
	file, err := root.OpenFile("period.lock", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.Join(errors.New("hosting lock changed"), file.Close())
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

func (lease *hostingLease) close() error {
	return errors.Join(syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN), lease.file.Close())
}

func measureHosting(names []string) (hostingReading, error) {
	var reading hostingReading
	boot, err := boundedFile("/proc/sys/kernel/random/boot_id", 64)
	if err != nil {
		return reading, err
	}
	reading.Boot = strings.TrimSpace(boot)
	if len(reading.Boot) != 36 {
		return reading, errors.New("hosting boot identity is unavailable")
	}
	for _, name := range names {
		directory := filepath.Join("/sys/class/net", name)
		values := make([]uint64, 3)
		for index, field := range []string{"ifindex", "statistics/tx_bytes", "statistics/rx_bytes"} {
			raw, err := boundedFile(filepath.Join(directory, field), 64)
			if err != nil {
				return hostingReading{}, err
			}
			values[index], err = strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
			if err != nil {
				return hostingReading{}, errors.New("hosting interface counter is unavailable")
			}
		}
		// Recheck identity after sampling: a removed/recreated device is no longer
		// the counter source bound to this period, even when its name is reused.
		after, err := boundedFile(filepath.Join(directory, "ifindex"), 64)
		if err != nil || strings.TrimSpace(after) != strconv.FormatUint(values[0], 10) {
			return hostingReading{}, errors.New("hosting interface changed during observation")
		}
		reading.Interfaces = append(reading.Interfaces, hostingInterface{Name: name, Index: values[0], Tx: values[1], Rx: values[2]})
	}
	return reading, nil
}

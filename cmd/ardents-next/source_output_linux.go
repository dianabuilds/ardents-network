//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"syscall"
	"time"
)

// Real command reporting uses finite nonblocking writes to its original stream.
// The plain writer branch is for in-process callers with no native descriptor.
// No detached output goroutine can outlive original State retirement.
func writeSourceReport(out io.Writer, raw []byte) error {
	return writeSourceReportContext(context.Background(), out, raw)
}

func writeSourceReportContext(parent context.Context, out io.Writer, raw []byte) error {
	if err := parent.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	file, ok := out.(*os.File)
	if !ok {
		n, err := out.Write(raw)
		if err == nil && n != len(raw) {
			err = io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeCharDevice|os.ModeSocket) == 0 {
		return errors.New("Source report requires interruptible output")
	}
	fd := int(file.Fd())
	defer runtime.KeepAlive(file)
	if err := syscall.SetNonblock(fd, true); err != nil {
		return err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for len(raw) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := syscall.Write(fd, raw)
		if n > 0 {
			raw = raw[n:]
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, syscall.EAGAIN) && !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return ctx.Err()
}

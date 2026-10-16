//go:build !windows

package node

import (
	"context"
	"errors"
	"os"
	"runtime"
	"syscall"
	"time"
)

func writeEvent(ctx context.Context, output *os.File, raw []byte) (int, error) {
	if _, ok := ctx.Deadline(); !ok {
		return 0, errors.New("node lifecycle event deadline is missing")
	}
	info, err := output.Stat()
	if err != nil || info.Mode()&(os.ModeNamedPipe|os.ModeCharDevice|os.ModeSocket) == 0 {
		return 0, errors.New("node lifecycle output is not an interruptible stream")
	}
	// Fd may restore blocking mode on a pollable File, so obtain it only once
	// before selecting nonblocking writes for this operation.
	fd := int(output.Fd())
	defer runtime.KeepAlive(output)
	if err := syscall.SetNonblock(fd, true); err != nil {
		return 0, err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	written := 0
	for written < len(raw) {
		count, writeErr := syscall.Write(fd, raw[written:])
		// A failed syscall can return -1; only actual bytes advance the offset.
		if count > 0 {
			written += count
		}
		if writeErr == nil {
			continue
		}
		if !errors.Is(writeErr, syscall.EAGAIN) && !errors.Is(writeErr, syscall.EWOULDBLOCK) && !errors.Is(writeErr, syscall.EINTR) {
			return written, writeErr
		}
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		case <-ticker.C:
		}
	}
	return written, nil
}

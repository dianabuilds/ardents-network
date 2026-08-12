//go:build !windows

package node

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

func writeEvent(ctx context.Context, output *os.File, raw []byte) (int, error) {
	if _, ok := ctx.Deadline(); !ok {
		return 0, errors.New("node lifecycle event deadline is missing")
	}
	info, err := output.Stat()
	if err != nil || info.Mode()&(os.ModeNamedPipe|os.ModeCharDevice) == 0 {
		return 0, errors.New("node lifecycle output is not an interruptible stream")
	}
	if err := syscall.SetNonblock(int(output.Fd()), true); err != nil {
		return 0, err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	written := 0
	for written < len(raw) {
		count, writeErr := syscall.Write(int(output.Fd()), raw[written:])
		written += count
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

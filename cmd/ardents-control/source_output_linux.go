package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"syscall"
	"time"
)

// The finite receipt contains no input paths or authority. Write on the actual
// output descriptor; a detached reporter cannot survive the original command.
func writeSourceControlOutcome(parent context.Context, out io.Writer, outcome string) error {
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Outcome   string `json:"outcome"`
	}{"source.prepare-state", outcome})
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return writeSourceControlBytes(parent, out, raw)
}

func writeSourceControlBytes(parent context.Context, out io.Writer, raw []byte) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	file, native := out.(*os.File)
	if !native {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := out.Write(raw)
		if err != nil || count != len(raw) {
			return errors.Join(io.ErrShortWrite, err)
		}
		return ctx.Err()
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeCharDevice|os.ModeSocket) == 0 {
		return errors.New("source control output requires interruptible native output")
	}
	fd := int(file.Fd())
	defer runtime.KeepAlive(file)
	if err := syscall.SetNonblock(fd, true); err != nil {
		return err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for len(raw) != 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		count, err := syscall.Write(fd, raw)
		if count > 0 {
			raw = raw[count:]
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

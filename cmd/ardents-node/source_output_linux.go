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

// Native reporting is bounded on the original descriptor. It joins here; no
// detached writer can outlive the original State or retain its key/material.
func writeInstalledSourceOutcome(parent context.Context, out io.Writer, outcome string) error {
	return writeSourceCommandOutcome(parent, out, "source.start-installed", outcome)
}

func writeSourceCommandOutcome(parent context.Context, out io.Writer, operation, outcome string) error {
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Outcome   string `json:"outcome"`
	}{operation, outcome})
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
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
		return errors.New("source output requires interruptible native output")
	}
	fd := int(file.Fd())
	defer runtime.KeepAlive(file)
	if err = syscall.SetNonblock(fd, true); err != nil {
		return err
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for len(raw) > 0 {
		if err = ctx.Err(); err != nil {
			return err
		}
		count, e := syscall.Write(fd, raw)
		if count > 0 {
			raw = raw[count:]
		}
		if e == nil {
			continue
		}
		if !errors.Is(e, syscall.EAGAIN) && !errors.Is(e, syscall.EWOULDBLOCK) && !errors.Is(e, syscall.EINTR) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return ctx.Err()
}

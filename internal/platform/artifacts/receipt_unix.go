//go:build linux || darwin

package artifacts

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"syscall"
	"time"
)

type outputKind uint8

const (
	receiptOutput outputKind = iota
	reportOutput
)

// WriteReceipt completes one bounded write to the caller-owned output.
func WriteReceipt(parent context.Context, out io.Writer, raw []byte) error {
	return writeOutput(parent, out, raw, receiptOutput)
}

// WriteReport writes a command report without replacing its original writer
// error with a receipt failure. An already canceled report never opens output.
func WriteReport(parent context.Context, out io.Writer, raw []byte) error {
	if err := parent.Err(); err != nil {
		return err
	}
	return writeOutput(parent, out, raw, reportOutput)
}

func writeOutput(parent context.Context, out io.Writer, raw []byte, kind outputKind) error {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	file, native := out.(*os.File)
	if !native {
		if kind == receiptOutput {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		count, err := out.Write(raw)
		switch {
		case kind == receiptOutput && (err != nil || count != len(raw)):
			return errors.Join(io.ErrShortWrite, err)
		case err != nil:
			return err
		case count != len(raw):
			return io.ErrShortWrite
		}
		return ctx.Err()
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&(os.ModeNamedPipe|os.ModeCharDevice|os.ModeSocket) == 0 {
		if kind == reportOutput {
			return errors.New("report for Source requires interruptible output")
		}
		return errors.New("artifact receipt requires interruptible native output")
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

// Refuse presents the selected public failure without disclosing native details.
func Refuse(cause error, message string) error {
	return &refusal{cause, message}
}

type refusal struct {
	cause   error
	message string
}

func (e *refusal) Error() string { return e.message }
func (e *refusal) Unwrap() error { return e.cause }

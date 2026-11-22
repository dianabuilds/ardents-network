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

// WriteReceipt completes one bounded write to the caller-owned output.
func WriteReceipt(parent context.Context, out io.Writer, raw []byte) error {
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

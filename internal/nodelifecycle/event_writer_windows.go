//go:build windows

package nodelifecycle

import (
	"context"
	"os"
	"syscall"
)

func writeEvent(ctx context.Context, output *os.File, raw []byte) (int, error) {
	type result struct {
		count int
		err   error
	}
	done := make(chan result, 1)
	go func() {
		count, err := output.Write(raw)
		done <- result{count: count, err: err}
	}()
	select {
	case value := <-done:
		return value.count, value.err
	case <-ctx.Done():
		_ = syscall.CancelIoEx(syscall.Handle(output.Fd()), nil)
		_ = output.Close()
		value := <-done
		return value.count, ctx.Err()
	}
}

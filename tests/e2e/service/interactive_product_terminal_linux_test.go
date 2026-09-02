//go:build linux

package service_test

import (
	"context"
	"errors"
	"syscall"
	"time"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

func interactiveProductLineEnding(bool) string {
	return "\n"
}

func waitForInteractiveNoEcho(ctx context.Context, terminal pty.Pty) error {
	unixTerminal, ok := terminal.(pty.UnixPty)
	if !ok {
		return errors.New("product pseudo-terminal lacks a Unix terminal endpoint")
	}
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := unix.IoctlGetTermios(int(unixTerminal.Slave().Fd()), unix.TCGETS)
		if err != nil {
			return err
		}
		if state.Lflag&unix.ECHO == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func isExpectedInteractiveProductReadError(err error) bool {
	return errors.Is(err, syscall.EIO)
}

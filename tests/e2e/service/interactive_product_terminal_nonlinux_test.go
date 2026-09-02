//go:build !linux

package service_test

import (
	"context"

	pty "github.com/aymanbagabas/go-pty"
)

func interactiveProductLineEnding(bool) string {
	return "\r\n"
}

func waitForInteractiveNoEcho(context.Context, pty.Pty) error {
	return nil
}

func isExpectedInteractiveProductReadError(error) bool {
	return false
}

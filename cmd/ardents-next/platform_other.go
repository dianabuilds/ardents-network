//go:build !linux

package main

import (
	"context"
	"io"
)

func runPlatform(_ context.Context, _ []string, out io.Writer) int {
	return unsupportedPlatformRefusal(out, "platform")
}

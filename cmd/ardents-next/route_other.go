//go:build !linux

package main

import (
	"context"
	"io"
)

func runRoute(_ context.Context, _ []string, out, _ io.Writer) int {
	return unsupportedPlatformRefusal(out, "route")
}

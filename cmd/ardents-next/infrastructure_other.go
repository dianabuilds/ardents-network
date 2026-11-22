//go:build !linux

package main

import (
	"context"
	"io"
)

func runInfrastructure(_ context.Context, _ []string, out io.Writer) int {
	return unsupportedPlatformRefusal(out, "infrastructure")
}

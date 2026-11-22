//go:build !linux

package main

import (
	"context"
	"io"
)

func runSourcePreparation(_ context.Context, _ []string, out io.Writer) int {
	return unsupportedPlatformRefusal(out, "source.prepare-trust")
}

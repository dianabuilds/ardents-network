//go:build !linux

package main

import (
	"context"
	"io"
)

func runExecutionTextRead(_ context.Context, _ holderPlan, _, _ [32]byte, _ string, out io.Writer) int {
	// No native qualification or participant effects on this platform.
	return unsupportedPlatformRefusal(out, "execution.text-read")
}

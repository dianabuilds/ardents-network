//go:build !linux

package main

import (
	"context"
	"io"
)

func runExecutionTextRead(context.Context, holderPlan, [32]byte, [32]byte, string, io.Writer) int {
	return 1 // No native qualification or participant effects on this platform.
}

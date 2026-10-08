//go:build !linux

package main

import (
	"context"
	"io"
)

func runInstalledEndpoint(ctx context.Context, args []string, _ io.Writer) int {
	if ctx == nil || len(args) != 2 || args[0] != "start-installed" {
		return 2
	}
	return 1
}

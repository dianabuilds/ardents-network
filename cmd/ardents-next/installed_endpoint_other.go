//go:build !linux

package main

import (
	"context"
	"io"
)

func runInstalledEndpoint(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 || !(args[0] == "start-installed" && len(args) == 2 || args[0] == "prepare-history" && len(args) == 4) {
		return 2
	}
	return unsupportedPlatformRefusal(diagnostic, "endpoint")
}

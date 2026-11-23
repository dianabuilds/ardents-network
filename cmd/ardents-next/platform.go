package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

const platformUnitTemplate = systemd.PlatformUnitTemplate

func runPlatform(ctx context.Context, args []string, out io.Writer) int {
	if ctx != nil && ctx.Err() == nil && len(args) == 1 && args[0] == "template" {
		if _, err := io.WriteString(out, platformUnitTemplate); err != nil {
			return 1
		}
		return 0
	}
	return runPlatformNative(ctx, args, out)
}

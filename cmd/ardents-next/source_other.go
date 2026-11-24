//go:build !linux

package main

import (
	"context"
	"io"
)

func runSourceCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return unsupportedPlatformRefusal(diagnostic, "source")
	}
	switch args[0] {
	case "prepare-trust":
		return unsupportedPlatformRefusal(out, "source.prepare-trust")
	case "prepare-state", "inspect-latest":
		return unsupportedPlatformRefusal(diagnostic, "source."+args[0])
	case "repair-retained":
		return runRepair(ctx, "source", args[1:], out, diagnostic)
	default:
		return unsupportedPlatformRefusal(diagnostic, "source")
	}
}

func runRepair(_ context.Context, role string, _ []string, out, diagnostic io.Writer) int {
	if role == "source" {
		return unsupportedPlatformRefusal(diagnostic, "source.repair-retained")
	}
	return unsupportedPlatformRefusal(out, "node."+role)
}

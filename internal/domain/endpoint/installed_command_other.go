//go:build !linux

package endpoint

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func RunInstalledCommand(ctx context.Context, args []string, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	operation := ""
	switch args[0] {
	case "start-installed":
		if len(args) != 2 {
			return 2
		}
		operation = "endpoint.start-installed"
	case "prepare-history":
		if len(args) != 4 {
			return 2
		}
		operation = "endpoint.prepare-history"
	case "construct-plan":
		if len(args) != 5 {
			return 2
		}
		operation = "endpoint.construct-plan"
	default:
		return 2
	}
	err := runtimeevent.WriteCommand(diagnostic, operation, "refused", "unsupported-platform")
	if err != nil {
		return 2
	}
	return 1
}

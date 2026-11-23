package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/launch"
)

func runRepair(ctx context.Context, role string, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || ctx.Err() != nil || len(args) != 1 {
		return 2
	}
	if err := launch.Repair(ctx, role, args[0]); err != nil {
		terminal := &sourceTerminal{FirstOwner: "state", First: err}
		if sourceFailureReport(diagnostic, "repair", terminal, nil) != nil {
			return 2
		}
		return 1
	}
	if _, err := io.WriteString(out, "repaired\n"); err != nil {
		return 2
	}
	return 0
}

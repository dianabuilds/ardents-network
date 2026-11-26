package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure/launch"
	sourcecommand "github.com/dianabuilds/ardents-network/internal/network/source/command"
)

func runSourceCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "repair-retained" {
		return launch.RunRepair(ctx, "source", args[1:], out, diagnostic)
	}
	return sourcecommand.Run(ctx, args, out, diagnostic)
}

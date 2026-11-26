package main

import (
	"context"
	"io"

	nodecommand "github.com/dianabuilds/ardents-network/internal/domain/node/command"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/runtimeplan"
	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure/launch"
)

func runNode(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	if args[0] == "inspect-process" {
		if len(args) != 1 {
			return 2
		}
		return nodecommand.InspectProcess(ctx, out)
	}
	if args[0] == "prepare-plan" {
		return runtimeplan.Run(ctx, args[1:], out)
	}
	operation, tail := args[0], args[1:]
	switch operation {
	case "forwarding", "descriptor", "introduction", "join":
		if len(tail) > 0 && tail[0] == "repair-retained" {
			return launch.RunRepair(ctx, operation, tail[1:], out, diagnostic)
		}
	case "issuer":
	default:
		return 2
	}
	return nodecommand.RunRole(ctx, operation, tail, out, diagnostic)
}

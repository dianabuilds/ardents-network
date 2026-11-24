package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func runNode(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	if args[0] == "inspect-process" {
		if len(args) != 1 {
			return 2
		}
		return inspectNodeProcess(ctx, out)
	}
	if args[0] == "prepare-plan" {
		return provisioning.RunPreparePlan(ctx, args[1:], out)
	}
	operation, tail := args[0], args[1:]
	var runRole func(context.Context, []string, io.Writer, io.Writer) int
	switch operation {
	case "forwarding":
		runRole = runNodeForwarding
	case "issuer":
		return runNodeIssuer(ctx, tail, out, diagnostic)
	case "descriptor":
		runRole = runNodeDescriptor
	case "introduction":
		runRole = runNodeIntroduction
	case "join":
		runRole = runNodeJoin
	default:
		return 2
	}
	if len(tail) > 0 && tail[0] == "repair-retained" {
		return runRepair(ctx, operation, tail[1:], out, diagnostic)
	}
	return runRole(ctx, tail, out, diagnostic)
}

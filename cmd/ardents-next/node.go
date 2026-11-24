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
	if len(args) == 1 && args[0] == "inspect-process" {
		return inspectNodeProcess(ctx, out)
	}
	if args[0] == "prepare-plan" {
		return provisioning.RunPreparePlan(ctx, args[1:], out)
	}
	if args[0] == "forwarding" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runRepair(ctx, "forwarding", args[2:], out, diagnostic)
		}
		return runNodeForwarding(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "issuer" {
		return runNodeIssuer(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "descriptor" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runRepair(ctx, "descriptor", args[2:], out, diagnostic)
		}
		return runNodeDescriptor(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "introduction" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runRepair(ctx, "introduction", args[2:], out, diagnostic)
		}
		return runNodeIntroduction(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "join" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runRepair(ctx, "join", args[2:], out, diagnostic)
		}
		return runNodeJoin(ctx, args[1:], out, diagnostic)
	}
	return 2
}

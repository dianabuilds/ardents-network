package main

import (
	"context"
	"io"
)

func runNode(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	if len(args) == 1 && args[0] == "inspect-process" {
		return inspectNodeProcess(ctx, out)
	}
	if args[0] == "prepare-plan" {
		return runNodePreparePlan(ctx, args[1:], out)
	}
	if args[0] == "forwarding" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runForwardingRepair(ctx, args[2:], out, diagnostic)
		}
		return runNodeForwarding(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "issuer" {
		return runNodeIssuer(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "descriptor" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runDescriptorRepair(ctx, args[2:], out, diagnostic)
		}
		return runNodeDescriptor(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "introduction" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runIntroductionRepair(ctx, args[2:], out, diagnostic)
		}
		return runNodeIntroduction(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "join" {
		if len(args) > 1 && args[1] == "repair-retained" {
			return runJoinRepair(ctx, args[2:], out, diagnostic)
		}
		return runNodeJoin(ctx, args[1:], out, diagnostic)
	}
	return 2
}

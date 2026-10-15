package main

import (
	"context"
	"encoding/json"
	"io"
)

func runNode(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) == 0 {
		return 2
	}
	if len(args) == 1 && args[0] == "inspect-process" {
		return inspectNodeProcess(ctx, out)
	}
	if args[0] == "forwarding" {
		return runNodeForwarding(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "issuer" {
		return runNodeIssuer(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "descriptor" {
		return runNodeDescriptor(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "introduction" {
		return runNodeIntroduction(ctx, args[1:], out, diagnostic)
	}
	if args[0] == "join" {
		return runNodeJoin(ctx, args[1:], out, diagnostic)
	}
	return 2
}

func nodeReport(out io.Writer, phase, outcome string) error {
	return json.NewEncoder(out).Encode(struct {
		Operation string `json:"operation"`
		Phase     string `json:"phase"`
		Outcome   string `json:"outcome"`
	}{"node.forwarding", phase, outcome})
}

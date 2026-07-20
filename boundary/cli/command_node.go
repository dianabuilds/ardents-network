package cli

import (
	"context"
	"flag"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) runNode(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderNodeUsage(a.stdout)
		return 0
	}
	switch args[0] {
	case "start":
		return a.nodeStart(ctx)
	case "stop":
		return a.nodeStop(ctx)
	case "status":
		return a.nodeStatus(ctx)
	case "runtime":
		return a.nodeRuntime(ctx)
	case "capabilities":
		return a.nodeCapabilities(ctx)
	case "events":
		return a.nodeEvents(ctx, args[1:])
	default:
		_, _ = fmt.Fprintf(a.stderr, "ard node: unknown subcommand %q\n", args[0])
		renderNodeUsage(a.stderr)
		return 2
	}
}

func (a *app) nodeStart(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().StartNode(callCtx, client.Request(a.cfg.Token, &ardentsv1.StartNodeRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "node start")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	return 0
}

func (a *app) nodeStop(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().StopNode(callCtx, client.Request(a.cfg.Token, &ardentsv1.StopNodeRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "node stop")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	return 0
}

func (a *app) nodeStatus(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetNodeStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNodeStatusRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "node status")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	node := resp.Msg.GetSnapshot().GetNode()
	printKV(a.stdout, "name", node.GetName())
	printKV(a.stdout, "state", node.GetState())
	printKV(a.stdout, "ready", boolString(node.GetReady()))
	printKV(a.stdout, "reason", node.GetReason())
	caps := resp.Msg.GetCapabilities()
	printKV(a.stdout, "version", caps.GetVersion())
	if services := caps.GetServices(); len(services) > 0 {
		printKV(a.stdout, "services", joinCSV(services))
	}
	return 0
}

func (a *app) nodeRuntime(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetNodeRuntime(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNodeRuntimeRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "node runtime")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	runtime := resp.Msg.GetRuntime()
	printKV(a.stdout, "node", runtime.GetNode().GetName())
	printKV(a.stdout, "state", runtime.GetNode().GetState())
	printKV(a.stdout, "boot_state", runtime.GetBoot().GetState())
	printKV(a.stdout, "boot_reason", runtime.GetBoot().GetReason())
	printKV(a.stdout, "identity_state", runtime.GetIdentity().GetState())
	printKV(a.stdout, "principal", runtime.GetIdentity().GetPrincipal())
	health := runtime.GetHealth()
	printKV(a.stdout, "health", health.GetState())
	if reason := health.GetPrimaryReason(); reason != nil {
		printKV(a.stdout, "health_reason", reason.GetSummary())
	}
	return 0
}

func (a *app) nodeCapabilities(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetNodeCapabilities(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNodeCapabilitiesRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "node capabilities")
	caps := resp.Msg.GetCapabilities()
	printKV(a.stdout, "version", caps.GetVersion())
	if services := caps.GetServices(); len(services) > 0 {
		printKV(a.stdout, "services", joinCSV(services))
	}
	return 0
}

func (a *app) nodeEvents(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("node events", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var limit int
	fs.IntVar(&limit, "limit", 0, "maximum number of events before exit")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	if !a.cfg.Watch {
		return a.fail(fmt.Errorf("node events requires --watch"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	stream, err := a.client.Service().StreamNodeEvents(callCtx, client.Request(a.cfg.Token, &ardentsv1.StreamNodeEventsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	return consumeNodeEvents(a.stdout, a.cfg.Output, stream, limit)
}

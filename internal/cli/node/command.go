// Package node owns node lifecycle command parsing and presentation.
// It does not own process lifecycle ownership.
package node

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type Command struct{ ctx commandctx.Context }

func New(ctx commandctx.Context) *Command { return &Command{ctx: ctx} }

func (a *Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderNodeUsage(a.ctx.Renderer.Out)
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
	case "features":
		return a.nodeFeatures(ctx)
	case "events":
		return a.nodeEvents(ctx, args[1:])
	default:
		output.Writef(a.ctx.Renderer.Err, "ardentsctl node: unknown subcommand %q\n", args[0])
		renderNodeUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func renderNodeUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] node <start|stop|status|runtime|features|events>")
}

func (a *Command) nodeStart(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().StartNode(callCtx, client.Request(&ardentsv1.StartNodeRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "node start")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	return 0
}

func (a *Command) nodeStop(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().StopNode(callCtx, client.Request(&ardentsv1.StopNodeRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "node stop")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	return 0
}

func (a *Command) nodeStatus(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetNodeStatus(callCtx, client.Request(&ardentsv1.GetNodeStatusRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "node status")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	node := resp.Msg.GetSnapshot().GetNode()
	output.KV(a.ctx.Renderer.Out, "name", node.GetName())
	output.KV(a.ctx.Renderer.Out, "state", node.GetState())
	output.KV(a.ctx.Renderer.Out, "ready", output.Bool(node.GetReady()))
	output.KV(a.ctx.Renderer.Out, "reason", node.GetReason())
	features := resp.Msg.GetFeatures()
	output.KV(a.ctx.Renderer.Out, "version", features.GetVersion())
	if services := features.GetServices(); len(services) > 0 {
		output.KV(a.ctx.Renderer.Out, "services", strings.Join(services, ", "))
	}
	return 0
}

func (a *Command) nodeRuntime(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetNodeRuntime(callCtx, client.Request(&ardentsv1.GetNodeRuntimeRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "node runtime")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	runtime := resp.Msg.GetRuntime()
	output.KV(a.ctx.Renderer.Out, "node", runtime.GetNode().GetName())
	output.KV(a.ctx.Renderer.Out, "state", runtime.GetNode().GetState())
	output.KV(a.ctx.Renderer.Out, "boot_state", runtime.GetBoot().GetState())
	output.KV(a.ctx.Renderer.Out, "boot_reason", runtime.GetBoot().GetReason())
	output.KV(a.ctx.Renderer.Out, "identity_state", runtime.GetIdentity().GetState())
	output.KV(a.ctx.Renderer.Out, "principal", runtime.GetIdentity().GetPrincipal())
	health := runtime.GetHealth()
	output.KV(a.ctx.Renderer.Out, "health", health.GetState())
	if reason := health.GetPrimaryReason(); reason != nil {
		output.KV(a.ctx.Renderer.Out, "health_reason", reason.GetSummary())
	}
	return 0
}

func (a *Command) nodeFeatures(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetNodeFeatures(callCtx, client.Request(&ardentsv1.GetNodeFeaturesRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "node features")
	features := resp.Msg.GetFeatures()
	output.KV(a.ctx.Renderer.Out, "version", features.GetVersion())
	if services := features.GetServices(); len(services) > 0 {
		output.KV(a.ctx.Renderer.Out, "services", strings.Join(services, ", "))
	}
	return 0
}

func (a *Command) nodeEvents(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("node events", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var limit int
	fs.IntVar(&limit, "limit", 0, "maximum number of events before exit")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	if !a.ctx.Watch {
		return a.ctx.Failure(fmt.Errorf("node events requires --watch"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	stream, err := a.ctx.Client.Service().StreamNodeEvents(callCtx, client.Request(&ardentsv1.StreamNodeEventsRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	return output.ConsumeEvents(a.ctx.Renderer, stream, limit)
}

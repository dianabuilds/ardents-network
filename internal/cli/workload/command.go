// Package workload owns workload and hosted-service command presentation.
// It does not own workload execution or hosting truth.
package workload

import (
	"context"
	"fmt"
	"io"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

type Command struct{ ctx commandctx.Context }

func New(ctx commandctx.Context) *Command { return &Command{ctx: ctx} }

func (a *Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderWorkloadUsage(a.ctx.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "list":
		return a.workloadList(ctx)
	case "get":
		return a.workloadGet(ctx, args[1:])
	case "register":
		return a.workloadRegister(ctx, args[1:])
	case "start":
		return a.workloadMutate(ctx, "start", args[1:])
	case "stop":
		return a.workloadMutate(ctx, "stop", args[1:])
	case "restart":
		return a.workloadMutate(ctx, "restart", args[1:])
	case "services":
		return a.workloadServices(ctx)
	case "service":
		return a.workloadService(ctx, args[1:])
	case "publication":
		return a.workloadPublication(ctx, args[1:])
	default:
		output.Writef(a.ctx.Renderer.Err, "ardentsctl workload: unknown subcommand %q\n", args[0])
		renderWorkloadUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func renderWorkloadUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] workload <list|get|register|start|stop|restart|services|service|publication>")
}

func (a *Command) workloadList(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListWorkloads(callCtx, client.Request(&ardentsv1.ListWorkloadsRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "workload list")
	if len(resp.Msg.GetWorkloads()) == 0 {
		output.Writeln(a.ctx.Renderer.Out, "no workloads")
		return 0
	}
	for _, item := range resp.Msg.GetWorkloads() {
		printWorkloadSummary(a.ctx.Renderer.Out, item)
	}
	return 0
}

func (a *Command) workloadGet(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("workload id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetWorkloadStatus(callCtx, client.Request(&ardentsv1.GetWorkloadStatusRequest{Id: id}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "workload get")
	printWorkloadSummary(a.ctx.Renderer.Out, resp.Msg)
	return 0
}

func (a *Command) workloadRegister(ctx context.Context, args []string) int {
	file, err := commandctx.ParseFileArg("workload register", a.ctx.Renderer.Err, args)
	if err != nil {
		return a.ctx.Failure(err)
	}
	spec := &ardentsv1.WorkloadSpecSnapshot{}
	if err := commandctx.LoadProtoJSON(a.ctx.Input, file, spec); err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().RegisterWorkload(callCtx, client.Request(&ardentsv1.RegisterWorkloadRequest{Spec: spec}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return a.ctx.Renderer.MutationOutcome(resp.Msg.GetStatus())
	}
	output.Header(a.ctx.Renderer.Out, "workload register")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	printWorkloadSummary(a.ctx.Renderer.Out, resp.Msg.GetWorkload())
	return a.ctx.Renderer.MutationOutcome(resp.Msg.GetStatus())
}

func (a *Command) workloadMutate(ctx context.Context, action string, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("workload id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	switch action {
	case "start":
		resp, err := a.ctx.Client.Service().StartWorkload(callCtx, client.Request(&ardentsv1.StartWorkloadRequest{Id: id}))
		if err != nil {
			return a.ctx.Failure(err)
		}
		return renderWorkloadCommand(a, "workload start", resp.Msg)
	case "stop":
		resp, err := a.ctx.Client.Service().StopWorkload(callCtx, client.Request(&ardentsv1.StopWorkloadRequest{Id: id}))
		if err != nil {
			return a.ctx.Failure(err)
		}
		return renderWorkloadCommand(a, "workload stop", resp.Msg)
	case "restart":
		resp, err := a.ctx.Client.Service().RestartWorkload(callCtx, client.Request(&ardentsv1.RestartWorkloadRequest{Id: id}))
		if err != nil {
			return a.ctx.Failure(err)
		}
		return renderWorkloadCommand(a, "workload restart", resp.Msg)
	default:
		return a.ctx.Failure(fmt.Errorf("unsupported action %q", action))
	}
}

func renderWorkloadCommand(a *Command, title string, msg *ardentsv1.WorkloadCommandResponse) int {
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, msg)
		return a.ctx.Renderer.MutationOutcome(msg.GetStatus())
	}
	output.Header(a.ctx.Renderer.Out, title)
	output.Status(a.ctx.Renderer.Out, msg.GetStatus())
	printWorkloadSummary(a.ctx.Renderer.Out, msg.GetWorkload())
	return a.ctx.Renderer.MutationOutcome(msg.GetStatus())
}

func printWorkloadSummary(w io.Writer, item *ardentsv1.WorkloadStatusSnapshot) {
	if item == nil {
		return
	}
	output.KV(w, "workload", item.GetSpec().GetId())
	output.KV(w, "  kind", item.GetSpec().GetKind())
	output.KV(w, "  desired", item.GetSpec().GetDesired())
	output.KV(w, "  observed", item.GetObserved())
	output.KV(w, "  reason", item.GetReason())
	output.KV(w, "  operator_action_required", output.Bool(item.GetNeedsOperatorAction()))
	output.KV(w, "  restart_count", fmt.Sprintf("%d", item.GetRestartCount()))
}

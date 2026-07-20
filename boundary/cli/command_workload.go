package cli

import (
	"context"
	"fmt"
	"io"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) runWorkload(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderWorkloadUsage(a.stdout)
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
		_, _ = fmt.Fprintf(a.stderr, "ard workload: unknown subcommand %q\n", args[0])
		renderWorkloadUsage(a.stderr)
		return 2
	}
}

func (a *app) workloadList(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListWorkloads(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListWorkloadsRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "workload list")
	if len(resp.Msg.GetWorkloads()) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "no workloads")
		return 0
	}
	for _, item := range resp.Msg.GetWorkloads() {
		printWorkloadSummary(a.stdout, item)
	}
	return 0
}

func (a *app) workloadGet(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("workload id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetWorkloadStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetWorkloadStatusRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "workload get")
	printWorkloadSummary(a.stdout, resp.Msg)
	return 0
}

func (a *app) workloadRegister(ctx context.Context, args []string) int {
	file, err := parseFileArg("workload register", a.stderr, args)
	if err != nil {
		return a.fail(err)
	}
	spec := &ardentsv1.WorkloadSpecSnapshot{}
	if err := loadProtoJSON(file, spec); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().RegisterWorkload(callCtx, client.Request(a.cfg.Token, &ardentsv1.RegisterWorkloadRequest{Spec: spec}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "workload register")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	printWorkloadSummary(a.stdout, resp.Msg.GetWorkload())
	return 0
}

func (a *app) workloadMutate(ctx context.Context, action string, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("workload id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	switch action {
	case "start":
		resp, err := a.client.Service().StartWorkload(callCtx, client.Request(a.cfg.Token, &ardentsv1.StartWorkloadRequest{Id: id}))
		if err != nil {
			return a.fail(err)
		}
		return renderWorkloadCommand(a, "workload start", resp.Msg)
	case "stop":
		resp, err := a.client.Service().StopWorkload(callCtx, client.Request(a.cfg.Token, &ardentsv1.StopWorkloadRequest{Id: id}))
		if err != nil {
			return a.fail(err)
		}
		return renderWorkloadCommand(a, "workload stop", resp.Msg)
	case "restart":
		resp, err := a.client.Service().RestartWorkload(callCtx, client.Request(a.cfg.Token, &ardentsv1.RestartWorkloadRequest{Id: id}))
		if err != nil {
			return a.fail(err)
		}
		return renderWorkloadCommand(a, "workload restart", resp.Msg)
	default:
		return a.fail(fmt.Errorf("unsupported action %q", action))
	}
}

func renderWorkloadCommand(a *app, title string, msg *ardentsv1.WorkloadCommandResponse) int {
	if a.jsonMode() {
		renderJSON(a.stdout, msg)
		return 0
	}
	printHeader(a.stdout, title)
	printStatusLine(a.stdout, msg.GetStatus())
	printWorkloadSummary(a.stdout, msg.GetWorkload())
	return 0
}

func printWorkloadSummary(w io.Writer, item *ardentsv1.WorkloadStatusSnapshot) {
	if item == nil {
		return
	}
	printKV(w, "workload", item.GetSpec().GetId())
	printKV(w, "  kind", item.GetSpec().GetKind())
	printKV(w, "  desired", item.GetSpec().GetDesired())
	printKV(w, "  observed", item.GetObserved())
	printKV(w, "  reason", item.GetReason())
	printKV(w, "  operator_action_required", boolString(item.GetNeedsOperatorAction()))
	printKV(w, "  restart_count", fmt.Sprintf("%d", item.GetRestartCount()))
}

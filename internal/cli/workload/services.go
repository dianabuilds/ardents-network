package workload

import (
	"context"
	"fmt"

	"ardents/internal/cli/client"
	commandctx "ardents/internal/cli/command"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func (a *Command) workloadServices(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListHostedServices(callCtx, client.Request(&ardentsv1.ListHostedServicesRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "workload services")
	if len(resp.Msg.GetServices()) == 0 {
		output.Writeln(a.ctx.Renderer.Out, "no hosted services")
		return 0
	}
	for _, item := range resp.Msg.GetServices() {
		output.KV(a.ctx.Renderer.Out, "service", item.GetId())
		output.KV(a.ctx.Renderer.Out, "  type", item.GetType())
		output.KV(a.ctx.Renderer.Out, "  workload", item.GetWorkloadId())
		output.KV(a.ctx.Renderer.Out, "  visibility", item.GetVisibility())
		output.KV(a.ctx.Renderer.Out, "  runtime_backing", item.GetRuntimeBacking())
		output.KV(a.ctx.Renderer.Out, "  readiness", item.GetReadiness())
		output.KV(a.ctx.Renderer.Out, "  ready", output.Bool(item.GetReady()))
		output.KV(a.ctx.Renderer.Out, "  exposure_eligible", output.Bool(item.GetExposureEligible()))
		output.KV(a.ctx.Renderer.Out, "  generation", fmt.Sprintf("%d", item.GetGeneration()))
	}
	return 0
}

func (a *Command) workloadService(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("service id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetHostedService(callCtx, client.Request(&ardentsv1.GetHostedServiceRequest{Id: id}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "workload service")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	service := resp.Msg.GetService()
	output.KV(a.ctx.Renderer.Out, "service_id", service.GetServiceId())
	output.KV(a.ctx.Renderer.Out, "state", service.GetState())
	output.KV(a.ctx.Renderer.Out, "reason", service.GetReason())
	output.KV(a.ctx.Renderer.Out, "published", output.Bool(service.GetPublished()))
	output.KV(a.ctx.Renderer.Out, "runtime_backing", service.GetRuntimeBacking())
	output.KV(a.ctx.Renderer.Out, "ready", output.Bool(service.GetReady()))
	output.KV(a.ctx.Renderer.Out, "exposure_eligible", output.Bool(service.GetExposureEligible()))
	output.KV(a.ctx.Renderer.Out, "generation", fmt.Sprintf("%d", service.GetGeneration()))
	return 0
}

func (a *Command) workloadPublication(ctx context.Context, args []string) int {
	id, ok := commandctx.FirstArg(args)
	if !ok {
		return a.ctx.Failure(fmt.Errorf("service id is required"))
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetServicePublicationStatus(callCtx, client.Request(&ardentsv1.GetServicePublicationStatusRequest{Id: id}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "workload publication")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	pub := resp.Msg.GetPublication()
	output.KV(a.ctx.Renderer.Out, "state", pub.GetState())
	output.KV(a.ctx.Renderer.Out, "reason", pub.GetReason())
	output.KV(a.ctx.Renderer.Out, "published", output.Bool(pub.GetPublished()))
	output.KV(a.ctx.Renderer.Out, "operator_action_required", output.Bool(pub.GetOperatorActionRequired()))
	return 0
}

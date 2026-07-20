package cli

import (
	"context"
	"fmt"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) workloadServices(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListHostedServices(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListHostedServicesRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "workload services")
	if len(resp.Msg.GetServices()) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "no hosted services")
		return 0
	}
	for _, item := range resp.Msg.GetServices() {
		printKV(a.stdout, "service", item.GetId())
		printKV(a.stdout, "  type", item.GetType())
		printKV(a.stdout, "  workload", item.GetWorkloadId())
		printKV(a.stdout, "  visibility", item.GetVisibility())
		printKV(a.stdout, "  runtime_backing", item.GetRuntimeBacking())
		printKV(a.stdout, "  readiness", item.GetReadiness())
		printKV(a.stdout, "  ready", boolString(item.GetReady()))
		printKV(a.stdout, "  exposure_eligible", boolString(item.GetExposureEligible()))
		printKV(a.stdout, "  generation", fmt.Sprintf("%d", item.GetGeneration()))
	}
	return 0
}

func (a *app) workloadService(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("service id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetHostedService(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetHostedServiceRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "workload service")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	service := resp.Msg.GetService()
	printKV(a.stdout, "service_id", service.GetServiceId())
	printKV(a.stdout, "state", service.GetState())
	printKV(a.stdout, "reason", service.GetReason())
	printKV(a.stdout, "published", boolString(service.GetPublished()))
	printKV(a.stdout, "runtime_backing", service.GetRuntimeBacking())
	printKV(a.stdout, "ready", boolString(service.GetReady()))
	printKV(a.stdout, "exposure_eligible", boolString(service.GetExposureEligible()))
	printKV(a.stdout, "generation", fmt.Sprintf("%d", service.GetGeneration()))
	return 0
}

func (a *app) workloadPublication(ctx context.Context, args []string) int {
	id, ok := firstArg(args)
	if !ok {
		return a.fail(fmt.Errorf("service id is required"))
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetServicePublicationStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetServicePublicationStatusRequest{Id: id}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "workload publication")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	pub := resp.Msg.GetPublication()
	printKV(a.stdout, "state", pub.GetState())
	printKV(a.stdout, "reason", pub.GetReason())
	printKV(a.stdout, "published", boolString(pub.GetPublished()))
	printKV(a.stdout, "operator_action_required", boolString(pub.GetOperatorActionRequired()))
	return 0
}

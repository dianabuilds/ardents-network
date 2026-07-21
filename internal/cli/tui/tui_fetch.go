package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ardents/internal/cli/client"
	diagnosticscmd "ardents/internal/cli/diagnostics"
	"ardents/internal/cli/output"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func (a *Command) fetchTUISnapshot(ctx context.Context, section tuiSection) (tuiSnapshot, error) {
	switch section {
	case tuiNode:
		return a.fetchTUINode(ctx)
	case tuiNetwork:
		return a.fetchTUINetwork(ctx)
	case tuiWorkloads:
		return a.fetchTUIWorkloads(ctx)
	case tuiData:
		return a.fetchTUIData(ctx)
	case tuiDiagnostics:
		return a.fetchTUIDiagnostics(ctx)
	default:
		return tuiSnapshot{}, fmt.Errorf("unsupported tui section %d", section)
	}
}

func (a *Command) fetchTUINode(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	status, err := a.ctx.Client.Service().GetNodeStatus(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetNodeStatusRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	runtime, err := a.ctx.Client.Service().GetNodeRuntime(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetNodeRuntimeRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	node := status.Msg.GetSnapshot().GetNode()
	rt := runtime.Msg.GetRuntime()
	return tuiSnapshot{
		Title:     "Node",
		UpdatedAt: time.Now(),
		Lines: []string{
			fmt.Sprintf("status: %s", status.Msg.GetStatus().GetState()),
			fmt.Sprintf("name: %s", node.GetName()),
			fmt.Sprintf("state: %s", node.GetState()),
			fmt.Sprintf("ready: %s", output.Bool(node.GetReady())),
			fmt.Sprintf("reason: %s", node.GetReason()),
			fmt.Sprintf("boot: %s", rt.GetBoot().GetState()),
			fmt.Sprintf("identity: %s", rt.GetIdentity().GetState()),
			fmt.Sprintf("health: %s", rt.GetHealth().GetState()),
		},
	}, nil
}

func (a *Command) fetchTUINetwork(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	network, err := a.ctx.Client.Service().GetNetworkStatus(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetNetworkStatusRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	discovery, err := a.ctx.Client.Service().GetDiscoveryStatus(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetDiscoveryStatusRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	presence, err := a.ctx.Client.Service().GetLocalPresence(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetLocalPresenceRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	snapshot := network.Msg.GetNetwork()
	return tuiSnapshot{
		Title:     "Network",
		UpdatedAt: time.Now(),
		Lines: []string{
			fmt.Sprintf("state: %s", snapshot.GetState()),
			fmt.Sprintf("reason: %s", snapshot.GetReason()),
			fmt.Sprintf("joined: %s", output.Bool(snapshot.GetJoined())),
			fmt.Sprintf("reachable: %s", output.Bool(snapshot.GetReachable())),
			fmt.Sprintf("profile: %s", snapshot.GetActiveProfile()),
			fmt.Sprintf("mode: %s", snapshot.GetActiveMode()),
			fmt.Sprintf("discovery: %s", discovery.Msg.GetDiscovery().GetState()),
			fmt.Sprintf("local presence: %s", presence.Msg.GetPresence().GetState()),
		},
	}, nil
}

func (a *Command) fetchTUIWorkloads(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	workloads, err := a.ctx.Client.Service().ListWorkloads(callCtx, client.Request(a.ctx.Token, &ardentsv1.ListWorkloadsRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	services, err := a.ctx.Client.Service().ListHostedServices(callCtx, client.Request(a.ctx.Token, &ardentsv1.ListHostedServicesRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	running := 0
	needsAction := 0
	for _, item := range workloads.Msg.GetWorkloads() {
		if item.GetObserved() == "running" {
			running++
		}
		if item.GetNeedsOperatorAction() {
			needsAction++
		}
	}
	lines := []string{
		fmt.Sprintf("workloads: %d", len(workloads.Msg.GetWorkloads())),
		fmt.Sprintf("running: %d", running),
		fmt.Sprintf("operator action required: %d", needsAction),
		fmt.Sprintf("hosted services: %d", len(services.Msg.GetServices())),
	}
	for _, item := range workloads.Msg.GetWorkloads() {
		lines = append(lines, fmt.Sprintf("- %s: observed=%s reason=%s", item.GetSpec().GetId(), item.GetObserved(), item.GetReason()))
		if len(lines) >= 7 {
			break
		}
	}
	return tuiSnapshot{Title: "Workloads", UpdatedAt: time.Now(), Lines: lines}, nil
}

func (a *Command) fetchTUIData(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	inventory, err := a.ctx.Client.Service().GetDataInventory(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetDataInventoryRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	transfers, err := a.ctx.Client.Service().ListTransfers(callCtx, client.Request(a.ctx.Token, &ardentsv1.ListTransfersRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	pending := 0
	for _, item := range transfers.Msg.GetTransfers() {
		if item.GetState() == "pending" || item.GetState() == "running" {
			pending++
		}
	}
	lines := []string{
		fmt.Sprintf("objects: %d", inventory.Msg.GetObjects()),
		fmt.Sprintf("manifests: %d", inventory.Msg.GetManifests()),
		fmt.Sprintf("blobs: %d", inventory.Msg.GetBlobs()),
		fmt.Sprintf("local blobs: %d", inventory.Msg.GetLocalBlobs()),
		fmt.Sprintf("remote blobs: %d", inventory.Msg.GetRemoteBlobs()),
		fmt.Sprintf("pending transfers: %d", pending),
	}
	for _, item := range transfers.Msg.GetTransfers() {
		lines = append(lines, fmt.Sprintf("- %s: %s %s", item.GetId(), item.GetState(), item.GetReason()))
		if len(lines) >= 9 {
			break
		}
	}
	return tuiSnapshot{Title: "Data", UpdatedAt: time.Now(), Lines: lines}, nil
}

func (a *Command) fetchTUIDiagnostics(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	health, err := a.ctx.Client.Service().GetHealthSummary(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetHealthSummaryRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	pending, err := a.ctx.Client.Service().GetPendingOperations(callCtx, client.Request(a.ctx.Token, &ardentsv1.GetPendingOperationsRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	reason := ""
	if item := health.Msg.GetHealth().GetPrimaryReason(); item != nil {
		reason = item.GetSummary()
	}
	lines := []string{
		fmt.Sprintf("state: %s", health.Msg.GetHealth().GetState()),
		fmt.Sprintf("reason: %s", reason),
		fmt.Sprintf("degraded domains: %s", strings.Join(diagnosticscmd.DegradedDomains(health.Msg.GetHealth()), ", ")),
		fmt.Sprintf("operator action required: %s", output.Bool(health.Msg.GetHealth().GetOperatorActionRequired())),
		fmt.Sprintf("pending operations: %d", len(pending.Msg.GetOperations())),
	}
	for _, item := range pending.Msg.GetOperations() {
		lines = append(lines, fmt.Sprintf("- %s: %s %s", item.GetId(), item.GetState(), item.GetReason()))
		if len(lines) >= 8 {
			break
		}
	}
	return tuiSnapshot{Title: "Diagnostics", UpdatedAt: time.Now(), Lines: lines}, nil
}

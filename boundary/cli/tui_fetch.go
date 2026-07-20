package cli

import (
	"context"
	"fmt"
	"time"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
)

func (a *app) fetchTUISnapshot(ctx context.Context, section tuiSection) (tuiSnapshot, error) {
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

func (a *app) fetchTUINode(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	status, err := a.client.Service().GetNodeStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNodeStatusRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	runtime, err := a.client.Service().GetNodeRuntime(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNodeRuntimeRequest{}))
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
			fmt.Sprintf("ready: %s", boolString(node.GetReady())),
			fmt.Sprintf("reason: %s", node.GetReason()),
			fmt.Sprintf("boot: %s", rt.GetBoot().GetState()),
			fmt.Sprintf("identity: %s", rt.GetIdentity().GetState()),
			fmt.Sprintf("health: %s", rt.GetHealth().GetState()),
		},
	}, nil
}

func (a *app) fetchTUINetwork(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	network, err := a.client.Service().GetNetworkStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNetworkStatusRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	discovery, err := a.client.Service().GetDiscoveryStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetDiscoveryStatusRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	presence, err := a.client.Service().GetLocalPresence(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetLocalPresenceRequest{}))
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
			fmt.Sprintf("joined: %s", boolString(snapshot.GetJoined())),
			fmt.Sprintf("reachable: %s", boolString(snapshot.GetReachable())),
			fmt.Sprintf("profile: %s", snapshot.GetActiveProfile()),
			fmt.Sprintf("mode: %s", snapshot.GetActiveMode()),
			fmt.Sprintf("discovery: %s", discovery.Msg.GetDiscovery().GetState()),
			fmt.Sprintf("local presence: %s", presence.Msg.GetPresence().GetState()),
		},
	}, nil
}

func (a *app) fetchTUIWorkloads(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	workloads, err := a.client.Service().ListWorkloads(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListWorkloadsRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	services, err := a.client.Service().ListHostedServices(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListHostedServicesRequest{}))
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

func (a *app) fetchTUIData(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	inventory, err := a.client.Service().GetDataInventory(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetDataInventoryRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	transfers, err := a.client.Service().ListTransfers(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListTransfersRequest{}))
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

func (a *app) fetchTUIDiagnostics(ctx context.Context) (tuiSnapshot, error) {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	health, err := a.client.Service().GetHealthSummary(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetHealthSummaryRequest{}))
	if err != nil {
		return tuiSnapshot{}, err
	}
	pending, err := a.client.Service().GetPendingOperations(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetPendingOperationsRequest{}))
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
		fmt.Sprintf("degraded domains: %s", joinCSV(degradedDomains(health.Msg.GetHealth()))),
		fmt.Sprintf("operator action required: %s", boolString(health.Msg.GetHealth().GetOperatorActionRequired())),
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

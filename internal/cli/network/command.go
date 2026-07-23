// Package network owns network and discovery command parsing and presentation.
// It does not own network participation or discovery state.
package network

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
		renderNetworkUsage(a.ctx.Renderer.Out)
		return 0
	}
	switch args[0] {
	case "status":
		return a.networkStatus(ctx)
	case "discovery":
		return a.discoveryStatus(ctx)
	case "presence":
		return a.localPresence(ctx)
	case "peers":
		return a.listPeers(ctx)
	case "routes":
		return a.listRoutes(ctx, args[1:])
	case "resolve":
		return a.resolve(ctx, args[1:])
	case "records":
		return a.records(ctx, args[1:])
	default:
		output.Writef(a.ctx.Renderer.Err, "ardentsctl network: unknown subcommand %q\n", args[0])
		renderNetworkUsage(a.ctx.Renderer.Err)
		return 2
	}
}

func renderNetworkUsage(writer io.Writer) {
	output.Writeln(writer, "Usage: ardentsctl [global flags] network <status|discovery|presence|peers|routes|resolve|records>")
}

func (a *Command) networkStatus(ctx context.Context) int {
	return commandctx.RunQuery(ctx, a.ctx, "network status", func(callCtx context.Context) (*ardentsv1.NetworkStatusResponse, error) {
		resp, err := a.ctx.Client.Service().GetNetworkStatus(callCtx, client.Request(&ardentsv1.GetNetworkStatusRequest{}))
		if err != nil {
			return nil, err
		}
		return resp.Msg, nil
	}, renderNetworkStatusHuman)
}

func renderNetworkStatusHuman(w io.Writer, msg *ardentsv1.NetworkStatusResponse) {
	output.Header(w, "network status")
	output.Status(w, msg.GetStatus())
	network := msg.GetNetwork()
	output.KV(w, "state", network.GetState())
	output.KV(w, "reason", network.GetReason())
	output.KV(w, "joined", output.Bool(network.GetJoined()))
	output.KV(w, "reachable", output.Bool(network.GetReachable()))
	output.KV(w, "profile", network.GetActiveProfile())
	output.KV(w, "mode", network.GetActiveMode())
	output.KV(w, "node_profile", network.GetNodeProfile())
	output.KV(w, "privacy_profile", network.GetPrivacyProfile())
	output.KV(w, "privacy_state", network.GetPrivacyState())
	output.KV(w, "privacy_switch_reason", network.GetPrivacySwitchReason())
	output.KV(w, "privacy_recovery_state", network.GetPrivacyRecoveryState())
	if reduced := network.GetReducedFeatures(); len(reduced) > 0 {
		output.KV(w, "reduced_features", strings.Join(reduced, ", "))
	}
	if active := network.GetActiveFeatures(); len(active) > 0 {
		output.KV(w, "active_features", strings.Join(active, ", "))
	}
	if categories := network.GetPrivacyErrorCategories(); len(categories) > 0 {
		output.KV(w, "privacy_error_categories", strings.Join(categories, ", "))
	}
}

func (a *Command) discoveryStatus(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetDiscoveryStatus(callCtx, client.Request(&ardentsv1.GetDiscoveryStatusRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network discovery")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	discovery := resp.Msg.GetDiscovery()
	output.KV(a.ctx.Renderer.Out, "state", discovery.GetState())
	output.KV(a.ctx.Renderer.Out, "reason", discovery.GetReason())
	output.KV(a.ctx.Renderer.Out, "local_records", fmt.Sprintf("%d", discovery.GetLocalRecords()))
	output.KV(a.ctx.Renderer.Out, "remote_records", fmt.Sprintf("%d", discovery.GetRemoteRecords()))
	output.KV(a.ctx.Renderer.Out, "trusted_records", fmt.Sprintf("%d", discovery.GetTrustedRecords()))
	output.KV(a.ctx.Renderer.Out, "rejected_records", fmt.Sprintf("%d", discovery.GetRejectedRecords()))
	output.KV(a.ctx.Renderer.Out, "stale_records", fmt.Sprintf("%d", discovery.GetStaleRecords()))
	return 0
}

func (a *Command) localPresence(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().GetLocalPresence(callCtx, client.Request(&ardentsv1.GetLocalPresenceRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network presence")
	output.Status(a.ctx.Renderer.Out, resp.Msg.GetStatus())
	presence := resp.Msg.GetPresence()
	output.KV(a.ctx.Renderer.Out, "published", output.Bool(presence.GetPublished()))
	output.KV(a.ctx.Renderer.Out, "state", presence.GetState())
	output.KV(a.ctx.Renderer.Out, "reason", presence.GetReason())
	output.KV(a.ctx.Renderer.Out, "record_id", presence.GetRecordId())
	output.KV(a.ctx.Renderer.Out, "operator_action_required", output.Bool(presence.GetOperatorActionRequired()))
	return 0
}

func (a *Command) listPeers(ctx context.Context) int {
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListPeers(callCtx, client.Request(&ardentsv1.ListPeersRequest{}))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network peers")
	if len(resp.Msg.GetPeers()) == 0 {
		output.Writeln(a.ctx.Renderer.Out, "no peers")
		return 0
	}
	for _, peer := range resp.Msg.GetPeers() {
		output.KV(a.ctx.Renderer.Out, "peer", peer.GetNodeId())
		output.KV(a.ctx.Renderer.Out, "  state", peer.GetState())
		output.KV(a.ctx.Renderer.Out, "  reason", peer.GetReason())
		output.KV(a.ctx.Renderer.Out, "  trust", peer.GetTrust().GetState())
		output.KV(a.ctx.Renderer.Out, "  usable", output.Bool(peer.GetTrust().GetUsable()))
		if addrs := peer.GetAddresses(); len(addrs) > 0 {
			output.KV(a.ctx.Renderer.Out, "  addresses", strings.Join(addrs, ", "))
		}
	}
	return 0
}

func (a *Command) listRoutes(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network routes", flag.ContinueOnError)
	fs.SetOutput(a.ctx.Renderer.Err)
	var req ardentsv1.ListRouteCandidatesRequest
	fs.StringVar(&req.Resource, "resource", "", "resource id")
	fs.StringVar(&req.Subject, "subject", "", "subject id")
	fs.StringVar(&req.Kind, "kind", "", "record kind")
	fs.StringVar(&req.Service, "service", "", "service id")
	if err := fs.Parse(args); err != nil {
		return a.ctx.Failure(err)
	}
	callCtx, cancel := a.ctx.Call(ctx)
	defer cancel()
	resp, err := a.ctx.Client.Service().ListRouteCandidates(callCtx, client.Request(&req))
	if err != nil {
		return a.ctx.Failure(err)
	}
	if a.ctx.Renderer.JSON {
		output.JSON(a.ctx.Renderer.Out, resp.Msg)
		return 0
	}
	output.Header(a.ctx.Renderer.Out, "network routes")
	route := resp.Msg.GetRoute()
	output.KV(a.ctx.Renderer.Out, "outcome", route.GetOutcome())
	output.KV(a.ctx.Renderer.Out, "reason", route.GetReason())
	if selected := route.GetSelected(); selected != nil {
		output.KV(a.ctx.Renderer.Out, "selected_endpoint", selected.GetEndpoint())
		output.KV(a.ctx.Renderer.Out, "selected_service", selected.GetService())
	}
	output.KV(a.ctx.Renderer.Out, "candidates", fmt.Sprintf("%d", route.GetCandidates()))
	output.KV(a.ctx.Renderer.Out, "usable", fmt.Sprintf("%d", route.GetUsable()))
	for _, item := range resp.Msg.GetCandidates() {
		output.KV(a.ctx.Renderer.Out, "candidate", item.GetEndpoint())
		output.KV(a.ctx.Renderer.Out, "  state", item.GetState())
		output.KV(a.ctx.Renderer.Out, "  reason", item.GetReason())
		output.KV(a.ctx.Renderer.Out, "  usable", output.Bool(item.GetUsable()))
		output.KV(a.ctx.Renderer.Out, "  trusted", output.Bool(item.GetTrusted()))
	}
	return 0
}

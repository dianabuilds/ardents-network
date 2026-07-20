package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"ardents/boundary/cli/client"
	ardentsv1 "ardents/proto/ardents/v1"
	"google.golang.org/protobuf/proto"
)

func (a *app) runNetwork(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] == "help" {
		renderNetworkUsage(a.stdout)
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
		_, _ = fmt.Fprintf(a.stderr, "ard network: unknown subcommand %q\n", args[0])
		renderNetworkUsage(a.stderr)
		return 2
	}
}

func (a *app) networkStatus(ctx context.Context) int {
	if a.cfg.Watch {
		return a.watchSnapshots(ctx, "network status", func(callCtx context.Context) (proto.Message, error) {
			resp, err := a.client.Service().GetNetworkStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNetworkStatusRequest{}))
			if err != nil {
				return nil, err
			}
			return resp.Msg, nil
		}, func(w io.Writer, msg proto.Message) {
			renderNetworkStatusHuman(w, msg.(*ardentsv1.NetworkStatusResponse))
		})
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetNetworkStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetNetworkStatusRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	renderNetworkStatusHuman(a.stdout, resp.Msg)
	return 0
}

func renderNetworkStatusHuman(w io.Writer, msg *ardentsv1.NetworkStatusResponse) {
	printHeader(w, "network status")
	printStatusLine(w, msg.GetStatus())
	network := msg.GetNetwork()
	printKV(w, "state", network.GetState())
	printKV(w, "reason", network.GetReason())
	printKV(w, "joined", boolString(network.GetJoined()))
	printKV(w, "reachable", boolString(network.GetReachable()))
	printKV(w, "profile", network.GetActiveProfile())
	printKV(w, "mode", network.GetActiveMode())
	printKV(w, "node_profile", network.GetNodeProfile())
	printKV(w, "privacy_profile", network.GetPrivacyProfile())
	printKV(w, "privacy_state", network.GetPrivacyState())
	printKV(w, "privacy_switch_reason", network.GetPrivacySwitchReason())
	printKV(w, "privacy_recovery_state", network.GetPrivacyRecoveryState())
	if reduced := network.GetReducedCapabilities(); len(reduced) > 0 {
		printKV(w, "reduced_capabilities", joinCSV(reduced))
	}
	if active := network.GetActiveCapabilities(); len(active) > 0 {
		printKV(w, "active_capabilities", joinCSV(active))
	}
	if categories := network.GetPrivacyErrorCategories(); len(categories) > 0 {
		printKV(w, "privacy_error_categories", joinCSV(categories))
	}
}

func (a *app) discoveryStatus(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetDiscoveryStatus(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetDiscoveryStatusRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network discovery")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	discovery := resp.Msg.GetDiscovery()
	printKV(a.stdout, "state", discovery.GetState())
	printKV(a.stdout, "reason", discovery.GetReason())
	printKV(a.stdout, "local_records", fmt.Sprintf("%d", discovery.GetLocalRecords()))
	printKV(a.stdout, "remote_records", fmt.Sprintf("%d", discovery.GetRemoteRecords()))
	printKV(a.stdout, "trusted_records", fmt.Sprintf("%d", discovery.GetTrustedRecords()))
	printKV(a.stdout, "rejected_records", fmt.Sprintf("%d", discovery.GetRejectedRecords()))
	printKV(a.stdout, "stale_records", fmt.Sprintf("%d", discovery.GetStaleRecords()))
	return 0
}

func (a *app) localPresence(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().GetLocalPresence(callCtx, client.Request(a.cfg.Token, &ardentsv1.GetLocalPresenceRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network presence")
	printStatusLine(a.stdout, resp.Msg.GetStatus())
	presence := resp.Msg.GetPresence()
	printKV(a.stdout, "published", boolString(presence.GetPublished()))
	printKV(a.stdout, "state", presence.GetState())
	printKV(a.stdout, "reason", presence.GetReason())
	printKV(a.stdout, "record_id", presence.GetRecordId())
	printKV(a.stdout, "operator_action_required", boolString(presence.GetOperatorActionRequired()))
	return 0
}

func (a *app) listPeers(ctx context.Context) int {
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListPeers(callCtx, client.Request(a.cfg.Token, &ardentsv1.ListPeersRequest{}))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network peers")
	if len(resp.Msg.GetPeers()) == 0 {
		_, _ = fmt.Fprintln(a.stdout, "no peers")
		return 0
	}
	for _, peer := range resp.Msg.GetPeers() {
		printKV(a.stdout, "peer", peer.GetNodeId())
		printKV(a.stdout, "  state", peer.GetState())
		printKV(a.stdout, "  reason", peer.GetReason())
		printKV(a.stdout, "  trust", peer.GetTrust().GetState())
		printKV(a.stdout, "  usable", boolString(peer.GetTrust().GetUsable()))
		if addrs := peer.GetAddresses(); len(addrs) > 0 {
			printKV(a.stdout, "  addresses", joinCSV(addrs))
		}
	}
	return 0
}

func (a *app) listRoutes(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("network routes", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	var req ardentsv1.ListRouteCandidatesRequest
	fs.StringVar(&req.Resource, "resource", "", "resource id")
	fs.StringVar(&req.Subject, "subject", "", "subject id")
	fs.StringVar(&req.Kind, "kind", "", "record kind")
	fs.StringVar(&req.Service, "service", "", "service id")
	if err := fs.Parse(args); err != nil {
		return a.fail(err)
	}
	callCtx, cancel := a.commandContext(ctx)
	defer cancel()
	resp, err := a.client.Service().ListRouteCandidates(callCtx, client.Request(a.cfg.Token, &req))
	if err != nil {
		return a.fail(err)
	}
	if a.jsonMode() {
		renderJSON(a.stdout, resp.Msg)
		return 0
	}
	printHeader(a.stdout, "network routes")
	route := resp.Msg.GetRoute()
	printKV(a.stdout, "outcome", route.GetOutcome())
	printKV(a.stdout, "reason", route.GetReason())
	if selected := route.GetSelected(); selected != nil {
		printKV(a.stdout, "selected_endpoint", selected.GetEndpoint())
		printKV(a.stdout, "selected_service", selected.GetService())
	}
	printKV(a.stdout, "candidates", fmt.Sprintf("%d", route.GetCandidates()))
	printKV(a.stdout, "usable", fmt.Sprintf("%d", route.GetUsable()))
	for _, item := range resp.Msg.GetCandidates() {
		printKV(a.stdout, "candidate", item.GetEndpoint())
		printKV(a.stdout, "  state", item.GetState())
		printKV(a.stdout, "  reason", item.GetReason())
		printKV(a.stdout, "  usable", boolString(item.GetUsable()))
		printKV(a.stdout, "  trusted", boolString(item.GetTrusted()))
	}
	return 0
}

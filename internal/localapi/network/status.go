package network

import (
	"context"

	discoveryapi "ardents/internal/discovery"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) GetDiscoveryStatus(ctx context.Context, _ *connect.Request[ardentsv1.GetDiscoveryStatusRequest]) (*connect.Response[ardentsv1.DiscoveryStatusResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.DiscoveryStatusResponse, *rpc.Error) {
		return &ardentsv1.DiscoveryStatusResponse{
			Status:    operationStatus("completed", "discovery status available", true),
			Discovery: toDiscoveryStatusSnapshot(h.status.GetDiscoveryStatus()),
		}, nil
	})
}

func (h *API) GetLocalPresence(ctx context.Context, _ *connect.Request[ardentsv1.GetLocalPresenceRequest]) (*connect.Response[ardentsv1.LocalPresenceResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.LocalPresenceResponse, *rpc.Error) {
		return &ardentsv1.LocalPresenceResponse{
			Status:   operationStatus("completed", "local presence available", true),
			Presence: toLocalPresenceSnapshot(h.status.GetLocalPresence()),
		}, nil
	})
}

func (h *API) ListPeers(ctx context.Context, _ *connect.Request[ardentsv1.ListPeersRequest]) (*connect.Response[ardentsv1.ListPeersResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.ListPeersResponse, *rpc.Error) {
		return &ardentsv1.ListPeersResponse{
			Status: operationStatus("completed", "peers available", true),
			Peers:  toPeerSnapshots(h.status.ListPeers()),
		}, nil
	})
}

func (h *API) ListRouteCandidates(ctx context.Context, req *connect.Request[ardentsv1.ListRouteCandidatesRequest]) (*connect.Response[ardentsv1.ListRouteCandidatesResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.ListRouteCandidatesResponse, *rpc.Error) {
		candidates, route, err := h.status.ListRouteCandidates(discoveryapi.ListRouteCandidatesQuery{
			Resource: req.Msg.GetResource(),
			Subject:  req.Msg.GetSubject(),
			Kind:     req.Msg.GetKind(),
			Service:  req.Msg.GetService(),
		})
		if err != nil {
			return nil, rpc.MapError("transport", "transport.route_candidates", "failed", "route candidate lookup failed", false, err)
		}
		return &ardentsv1.ListRouteCandidatesResponse{
			Status:     operationStatus("completed", "route candidates available", true),
			Candidates: toRouteCandidateSnapshots(candidates),
			Route:      toRouteSnapshot(route),
		}, nil
	})
}

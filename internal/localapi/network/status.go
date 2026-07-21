package network

import (
	"context"

	discoveryapi "ardents/internal/discovery"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) GetDiscoveryStatus(_ context.Context, req *connect.Request[ardentsv1.GetDiscoveryStatusRequest]) (*connect.Response[ardentsv1.DiscoveryStatusResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.DiscoveryStatusResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "discovery", "discovery.status"); err != nil {
			return nil, err
		}
		return &ardentsv1.DiscoveryStatusResponse{
			Status:    operationStatus("completed", "discovery status available", true),
			Discovery: toDiscoveryStatusSnapshot(h.status.GetDiscoveryStatus()),
		}, nil
	})
}

func (h *API) GetLocalPresence(_ context.Context, req *connect.Request[ardentsv1.GetLocalPresenceRequest]) (*connect.Response[ardentsv1.LocalPresenceResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.LocalPresenceResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "discovery", "discovery.local_presence"); err != nil {
			return nil, err
		}
		return &ardentsv1.LocalPresenceResponse{
			Status:   operationStatus("completed", "local presence available", true),
			Presence: toLocalPresenceSnapshot(h.status.GetLocalPresence()),
		}, nil
	})
}

func (h *API) ListPeers(_ context.Context, req *connect.Request[ardentsv1.ListPeersRequest]) (*connect.Response[ardentsv1.ListPeersResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListPeersResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "discovery", "discovery.peers"); err != nil {
			return nil, err
		}
		return &ardentsv1.ListPeersResponse{
			Status: operationStatus("completed", "peers available", true),
			Peers:  toPeerSnapshots(h.status.ListPeers()),
		}, nil
	})
}

func (h *API) ListRouteCandidates(_ context.Context, req *connect.Request[ardentsv1.ListRouteCandidatesRequest]) (*connect.Response[ardentsv1.ListRouteCandidatesResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListRouteCandidatesResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "transport", "transport.route_candidates"); err != nil {
			return nil, err
		}
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

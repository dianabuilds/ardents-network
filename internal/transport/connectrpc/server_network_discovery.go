package connectrpc

import (
	"context"

	nodeapi "ardents/internal/node/api"
	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetDiscoveryStatus(ctx context.Context, req *connect.Request[ardentsv1.GetDiscoveryStatusRequest]) (*connect.Response[ardentsv1.DiscoveryStatusResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.DiscoveryStatusResponse, *rpcError) {
		if err := requireRead(call, "discovery", "discovery.status"); err != nil {
			return nil, err
		}
		return &ardentsv1.DiscoveryStatusResponse{
			Status:    statusProto("completed", "discovery status available", true),
			Discovery: toDiscoveryStatusSnapshot(s.node.GetDiscoveryStatus()),
		}, nil
	})
}

func (s *Server) GetLocalPresence(ctx context.Context, req *connect.Request[ardentsv1.GetLocalPresenceRequest]) (*connect.Response[ardentsv1.LocalPresenceResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.LocalPresenceResponse, *rpcError) {
		if err := requireRead(call, "discovery", "discovery.local_presence"); err != nil {
			return nil, err
		}
		return &ardentsv1.LocalPresenceResponse{
			Status:   statusProto("completed", "local presence available", true),
			Presence: toLocalPresenceSnapshot(s.node.GetLocalPresence()),
		}, nil
	})
}

func (s *Server) ListPeers(ctx context.Context, req *connect.Request[ardentsv1.ListPeersRequest]) (*connect.Response[ardentsv1.ListPeersResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListPeersResponse, *rpcError) {
		if err := requireRead(call, "discovery", "discovery.peers"); err != nil {
			return nil, err
		}
		return &ardentsv1.ListPeersResponse{
			Status: statusProto("completed", "peers available", true),
			Peers:  toPeerSnapshots(s.node.ListPeers()),
		}, nil
	})
}

func (s *Server) ListRouteCandidates(ctx context.Context, req *connect.Request[ardentsv1.ListRouteCandidatesRequest]) (*connect.Response[ardentsv1.ListRouteCandidatesResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListRouteCandidatesResponse, *rpcError) {
		if err := requireRead(call, "transport", "transport.route_candidates"); err != nil {
			return nil, err
		}
		candidates, route, err := s.node.ListRouteCandidates(nodeapi.ListRouteCandidatesQuery{
			Resource: req.Msg.GetResource(),
			Subject:  req.Msg.GetSubject(),
			Kind:     req.Msg.GetKind(),
			Service:  req.Msg.GetService(),
		})
		if err != nil {
			return nil, mapAPIError("transport", "transport.route_candidates", "failed", "route candidate lookup failed", false, err)
		}
		return &ardentsv1.ListRouteCandidatesResponse{
			Status:     statusProto("completed", "route candidates available", true),
			Candidates: toRouteCandidateSnapshots(candidates),
			Route:      toRouteSnapshot(route),
		}, nil
	})
}

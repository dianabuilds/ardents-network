package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetNodeRuntime(ctx context.Context, req *connect.Request[ardentsv1.GetNodeRuntimeRequest]) (*connect.Response[ardentsv1.NodeRuntimeResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.NodeRuntimeResponse, *rpcError) {
		if err := requireRead(call, "node", "node.runtime"); err != nil {
			return nil, err
		}
		return &ardentsv1.NodeRuntimeResponse{
			Status:  statusProto("completed", "node runtime available", true),
			Runtime: toNodeRuntimeSnapshot(s.node.GetNodeRuntime()),
		}, nil
	})
}

func (s *Server) GetNetworkStatus(ctx context.Context, req *connect.Request[ardentsv1.GetNetworkStatusRequest]) (*connect.Response[ardentsv1.NetworkStatusResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.NetworkStatusResponse, *rpcError) {
		if err := requireRead(call, "transport", "transport.network_status"); err != nil {
			return nil, err
		}
		return &ardentsv1.NetworkStatusResponse{
			Status:  statusProto("completed", "network status available", true),
			Network: toNetworkStatusSnapshot(s.node.GetNetworkStatus()),
		}, nil
	})
}

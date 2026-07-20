package connectrpc

import (
	"context"

	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) StartNode(ctx context.Context, req *connect.Request[ardents.StartNodeRequest]) (*connect.Response[ardents.CommandAckResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.CommandAckResponse, *rpcError) {
		if err := requireWrite(call, "node", "node.start"); err != nil {
			return nil, err
		}
		if err := s.node.Start(callCtx); err != nil {
			return nil, mapAPIError("node", "node.start", "start_failed", "node start failed", true, err)
		}
		return &ardents.CommandAckResponse{Status: statusProto("completed", "node started", true)}, nil
	})
}

func (s *Server) StopNode(ctx context.Context, req *connect.Request[ardents.StopNodeRequest]) (*connect.Response[ardents.CommandAckResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.CommandAckResponse, *rpcError) {
		if err := requireWrite(call, "node", "node.stop"); err != nil {
			return nil, err
		}
		if err := s.node.Stop(callCtx); err != nil {
			return nil, mapAPIError("node", "node.stop", "stop_failed", "node stop failed", true, err)
		}
		return &ardents.CommandAckResponse{Status: statusProto("completed", "node stopped", true)}, nil
	})
}

package node

import (
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *RuntimeHandler) GetNodeRuntime(_ context.Context, req *connect.Request[ardentsv1.GetNodeRuntimeRequest]) (*connect.Response[ardentsv1.NodeRuntimeResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.NodeRuntimeResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "node", "node.runtime"); err != nil {
			return nil, err
		}
		return &ardentsv1.NodeRuntimeResponse{
			Status:  statusProto("completed", "node runtime available", true),
			Runtime: toNodeRuntimeSnapshot(h.service.GetNodeRuntime()),
		}, nil
	})
}

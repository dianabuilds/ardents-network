package node

import (
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *RuntimeHandler) GetNodeRuntime(ctx context.Context, _ *connect.Request[ardentsv1.GetNodeRuntimeRequest]) (*connect.Response[ardentsv1.NodeRuntimeResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.NodeRuntimeResponse, *rpc.Error) {
		return &ardentsv1.NodeRuntimeResponse{
			Status:  statusProto("completed", "node runtime available", true),
			Runtime: toNodeRuntimeSnapshot(h.service.GetNodeRuntime()),
		}, nil
	})
}

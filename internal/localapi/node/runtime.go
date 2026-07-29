package node

import (
	"context"
	"time"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (h *RuntimeHandler) GetNodeRuntime(ctx context.Context, _ *connect.Request[ardentsv1.GetNodeRuntimeRequest]) (*connect.Response[ardentsv1.NodeRuntimeResponse], error) {
	return rpc.RespondContext(ctx, func(call rpc.Call) (*ardentsv1.NodeRuntimeResponse, *rpc.Error) {
		runtime := toNodeRuntimeSnapshot(h.service.GetNodeRuntime())
		if _, admitted := call.Authorized(); admitted {
			runtime.Readiness.Checks = append([]*ardentsv1.ReadinessCheckSnapshot{
				{Name: "protected_api", Ready: true},
				{Name: "access_grant", Ready: true},
			}, runtime.Readiness.Checks...)
		}
		return &ardentsv1.NodeRuntimeResponse{
			Status:     statusProto("completed", "node runtime available", true),
			Runtime:    runtime,
			ObservedAt: timestamppb.New(time.Now().UTC()),
		}, nil
	})
}

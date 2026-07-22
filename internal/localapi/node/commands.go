// Package node owns process-level node protocol handlers and mappings.
// It does not own module state machines.
package node

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *RuntimeHandler) StartNode(ctx context.Context, req *connect.Request[ardents.StartNodeRequest]) (*connect.Response[ardents.CommandAckResponse], error) {
	return h.mutateNode(ctx, "start", "started", h.service.Start)
}

func (h *RuntimeHandler) StopNode(ctx context.Context, req *connect.Request[ardents.StopNodeRequest]) (*connect.Response[ardents.CommandAckResponse], error) {
	return h.mutateNode(ctx, "stop", "stopped", h.service.Stop)
}

func (h *RuntimeHandler) mutateNode(ctx context.Context, action, completedAction string,
	mutate func(context.Context) error,
) (*connect.Response[ardents.CommandAckResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	operation := "node." + action
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.CommandAckResponse, *rpc.Error) {
		if err := mutate(callCtx); err != nil {
			return nil, rpc.MapError("node", operation, action+"_failed", "node "+action+" failed", true, err)
		}
		return &ardents.CommandAckResponse{Status: statusProto("completed", "node "+completedAction, true)}, nil
	})
}

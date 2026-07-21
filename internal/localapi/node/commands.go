// Package node owns process-level node protocol handlers and mappings.
// It does not own module state machines.
package node

import (
	"context"
	"net/http"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *RuntimeHandler) StartNode(ctx context.Context, req *connect.Request[ardents.StartNodeRequest]) (*connect.Response[ardents.CommandAckResponse], error) {
	return h.mutateNode(ctx, req.Header(), "start", "started", h.service.Start)
}

func (h *RuntimeHandler) StopNode(ctx context.Context, req *connect.Request[ardents.StopNodeRequest]) (*connect.Response[ardents.CommandAckResponse], error) {
	return h.mutateNode(ctx, req.Header(), "stop", "stopped", h.service.Stop)
}

func (h *RuntimeHandler) mutateNode(ctx context.Context, header http.Header, action, completedAction string,
	mutate func(context.Context) error,
) (*connect.Response[ardents.CommandAckResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	operation := "node." + action
	return rpc.Respond(h.auth, header, func(call rpc.CallContext) (*ardents.CommandAckResponse, *rpc.Error) {
		if err := rpc.RequireWrite(call, "node", operation); err != nil {
			return nil, err
		}
		if err := mutate(callCtx); err != nil {
			return nil, rpc.MapError("node", operation, action+"_failed", "node "+action+" failed", true, err)
		}
		return &ardents.CommandAckResponse{Status: statusProto("completed", "node "+completedAction, true)}, nil
	})
}

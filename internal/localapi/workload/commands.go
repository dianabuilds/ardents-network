// Package workload owns workload and hosting protocol handlers and mappings.
// It does not own workload or hosting ownership.
package workload

import (
	"context"
	"net/http"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Service) RegisterWorkload(ctx context.Context, req *connect.Request[ardents.RegisterWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.WorkloadCommandResponse, *rpc.Error) {
		if err := rpc.RequireWrite(call, "workload", "workload.register"); err != nil {
			return nil, err
		}
		if err := h.workload.Register(callCtx, fromWorkloadSpecSnapshot(req.Msg.GetSpec())); err != nil {
			return nil, rpc.MapError("workload", "workload.register", "register_failed", "workload register failed", false, err)
		}
		workload, err := h.workload.Get(req.Msg.GetSpec().GetId())
		if err != nil {
			return nil, rpc.MapError("workload", "workload.register", "failed", "workload lookup after register failed", false, err)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload registered", true),
			Workload: toWorkloadStatusSnapshot(workload),
		}, nil
	})
}

func (h *Service) StartWorkload(ctx context.Context, req *connect.Request[ardents.StartWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	return h.mutateWorkload(ctx, req.Header(), req.Msg.GetId(), "start", "started", h.workload.Start)
}

func (h *Service) StopWorkload(ctx context.Context, req *connect.Request[ardents.StopWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	return h.mutateWorkload(ctx, req.Header(), req.Msg.GetId(), "stop", "stopped", h.workload.Stop)
}

func (h *Service) RestartWorkload(ctx context.Context, req *connect.Request[ardents.RestartWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	return h.mutateWorkload(ctx, req.Header(), req.Msg.GetId(), "restart", "restarted", h.workload.Restart)
}

func (h *Service) mutateWorkload(ctx context.Context, header http.Header, id, action, completedAction string,
	mutate func(context.Context, string) error,
) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	operation := "workload." + action
	return rpc.Respond(h.auth, header, func(call rpc.CallContext) (*ardents.WorkloadCommandResponse, *rpc.Error) {
		if err := rpc.RequireWrite(call, "workload", operation); err != nil {
			return nil, err
		}
		if err := mutate(callCtx, id); err != nil {
			return nil, rpc.MapError("workload", operation, action+"_failed", "workload "+action+" failed", true, err)
		}
		workload, err := h.workload.Get(id)
		if err != nil {
			return nil, rpc.MapError("workload", operation, "failed", "workload lookup after "+action+" failed", false, err)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload "+completedAction, true),
			Workload: toWorkloadStatusSnapshot(workload),
		}, nil
	})
}

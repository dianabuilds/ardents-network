// Package workload owns workload and hosting protocol handlers and mappings.
// It does not own workload or hosting ownership.
package workload

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Service) RegisterWorkload(ctx context.Context, req *connect.Request[ardents.RegisterWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.WorkloadCommandResponse, *rpc.Error) {
		spec, specErr := fromWorkloadSpecSnapshot(req.Msg.GetSpec())
		if specErr != nil {
			return nil, rpc.MapError("workload", "workload.register", "invalid_request", "workload specification is invalid", false, specErr)
		}
		if err := h.workload.Register(callCtx, spec); err != nil {
			return nil, rpc.MapError("workload", "workload.register", "register_failed", "workload register failed", false, err)
		}
		workload, err := h.workload.Get(req.Msg.GetSpec().GetId())
		if err != nil {
			return nil, rpc.MapError("workload", "workload.register", "failed", "workload lookup after register failed", false, err)
		}
		snapshot, snapshotErr := toWorkloadStatusSnapshot(workload)
		if snapshotErr != nil {
			return nil, rpc.MapError("workload", "workload.register", "invalid_state", "workload state is invalid", false, snapshotErr)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload registered", true),
			Workload: snapshot,
		}, nil
	})
}

func (h *Service) StartWorkload(ctx context.Context, req *connect.Request[ardents.StartWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	return h.mutateWorkload(ctx, req.Msg.GetId(), "start", "started", h.workload.Start)
}

func (h *Service) StopWorkload(ctx context.Context, req *connect.Request[ardents.StopWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	return h.mutateWorkload(ctx, req.Msg.GetId(), "stop", "stopped", h.workload.Stop)
}

func (h *Service) RestartWorkload(ctx context.Context, req *connect.Request[ardents.RestartWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	return h.mutateWorkload(ctx, req.Msg.GetId(), "restart", "restarted", h.workload.Restart)
}

func (h *Service) mutateWorkload(ctx context.Context, id, action, completedAction string,
	mutate func(context.Context, string) error,
) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := rpc.MutationContext(ctx)
	defer cancel()
	operation := "workload." + action
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.WorkloadCommandResponse, *rpc.Error) {
		if err := mutate(callCtx, id); err != nil {
			return nil, rpc.MapError("workload", operation, action+"_failed", "workload "+action+" failed", true, err)
		}
		workload, err := h.workload.Get(id)
		if err != nil {
			return nil, rpc.MapError("workload", operation, "failed", "workload lookup after "+action+" failed", false, err)
		}
		snapshot, snapshotErr := toWorkloadStatusSnapshot(workload)
		if snapshotErr != nil {
			return nil, rpc.MapError("workload", operation, "invalid_state", "workload state is invalid", false, snapshotErr)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload "+completedAction, true),
			Workload: snapshot,
		}, nil
	})
}

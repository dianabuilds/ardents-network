package workload

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Service) GetWorkloadStatus(ctx context.Context, req *connect.Request[ardents.GetWorkloadStatusRequest]) (*connect.Response[ardents.WorkloadStatusSnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.WorkloadStatusSnapshot, *rpc.Error) {
		res, err := h.workload.Get(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("workload", "workload.status", "failed", "workload status failed", false, err)
		}
		snapshot, snapshotErr := toWorkloadStatusSnapshot(res)
		if snapshotErr != nil {
			return nil, rpc.MapError("workload", "workload.status", "invalid_state", "workload state is invalid", false, snapshotErr)
		}
		return snapshot, nil
	})
}

func (h *Service) ListWorkloads(ctx context.Context, _ *connect.Request[ardents.ListWorkloadsRequest]) (*connect.Response[ardents.ListWorkloadsResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.ListWorkloadsResponse, *rpc.Error) {
		items, err := h.workload.List()
		if err != nil {
			return nil, rpc.MapError("workload", "workload.list", "failed", "workload list failed", false, err)
		}
		out := make([]*ardents.WorkloadStatusSnapshot, 0, len(items))
		for _, item := range items {
			snapshot, snapshotErr := toWorkloadStatusSnapshot(item)
			if snapshotErr != nil {
				return nil, rpc.MapError("workload", "workload.list", "invalid_state", "workload state is invalid", false, snapshotErr)
			}
			out = append(out, snapshot)
		}
		return &ardents.ListWorkloadsResponse{Workloads: out}, nil
	})
}

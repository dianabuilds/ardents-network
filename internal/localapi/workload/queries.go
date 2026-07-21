package workload

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Service) GetWorkloadStatus(_ context.Context, req *connect.Request[ardents.GetWorkloadStatusRequest]) (*connect.Response[ardents.WorkloadStatusSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.WorkloadStatusSnapshot, *rpc.Error) {
		if err := rpc.RequireRead(call, "workload", "workload.status"); err != nil {
			return nil, err
		}
		res, err := h.workload.Get(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("workload", "workload.status", "failed", "workload status failed", false, err)
		}
		return toWorkloadStatusSnapshot(res), nil
	})
}

func (h *Service) ListWorkloads(_ context.Context, req *connect.Request[ardents.ListWorkloadsRequest]) (*connect.Response[ardents.ListWorkloadsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.ListWorkloadsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "workload", "workload.list"); err != nil {
			return nil, err
		}
		items, err := h.workload.List()
		if err != nil {
			return nil, rpc.MapError("workload", "workload.list", "failed", "workload list failed", false, err)
		}
		out := make([]*ardents.WorkloadStatusSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toWorkloadStatusSnapshot(item))
		}
		return &ardents.ListWorkloadsResponse{Workloads: out}, nil
	})
}

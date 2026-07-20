package connectrpc

import (
	"context"

	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetWorkloadStatus(ctx context.Context, req *connect.Request[ardents.GetWorkloadStatusRequest]) (*connect.Response[ardents.WorkloadStatusSnapshot], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.WorkloadStatusSnapshot, *rpcError) {
		if err := requireRead(call, "workload", "workload.status"); err != nil {
			return nil, err
		}
		res, err := s.workload.GetWorkloadStatus(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.status", "failed", "workload status failed", false, err)
		}
		return toWorkloadStatusSnapshot(res), nil
	})
}

func (s *Server) ListWorkloads(ctx context.Context, req *connect.Request[ardents.ListWorkloadsRequest]) (*connect.Response[ardents.ListWorkloadsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.ListWorkloadsResponse, *rpcError) {
		if err := requireRead(call, "workload", "workload.list"); err != nil {
			return nil, err
		}
		items, err := s.workload.ListWorkloads()
		if err != nil {
			return nil, mapAPIError("workload", "workload.list", "failed", "workload list failed", false, err)
		}
		out := make([]*ardents.WorkloadStatusSnapshot, 0, len(items))
		for _, item := range items {
			out = append(out, toWorkloadStatusSnapshot(item))
		}
		return &ardents.ListWorkloadsResponse{Workloads: out}, nil
	})
}

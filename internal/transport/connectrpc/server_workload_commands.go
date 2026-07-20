package connectrpc

import (
	"context"

	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) RegisterWorkload(ctx context.Context, req *connect.Request[ardents.RegisterWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.WorkloadCommandResponse, *rpcError) {
		if err := requireWrite(call, "workload", "workload.register"); err != nil {
			return nil, err
		}
		if err := s.workload.RegisterWorkloadContext(callCtx, fromWorkloadSpecSnapshot(req.Msg.GetSpec())); err != nil {
			return nil, mapAPIError("workload", "workload.register", "register_failed", "workload register failed", false, err)
		}
		workload, err := s.workload.GetWorkloadStatus(req.Msg.GetSpec().GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.register", "failed", "workload lookup after register failed", false, err)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload registered", true),
			Workload: toWorkloadStatusSnapshot(workload),
		}, nil
	})
}

func (s *Server) StartWorkload(ctx context.Context, req *connect.Request[ardents.StartWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.WorkloadCommandResponse, *rpcError) {
		if err := requireWrite(call, "workload", "workload.start"); err != nil {
			return nil, err
		}
		if err := s.workload.StartWorkloadContext(callCtx, req.Msg.GetId()); err != nil {
			return nil, mapAPIError("workload", "workload.start", "start_failed", "workload start failed", true, err)
		}
		workload, err := s.workload.GetWorkloadStatus(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.start", "failed", "workload lookup after start failed", false, err)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload started", true),
			Workload: toWorkloadStatusSnapshot(workload),
		}, nil
	})
}

func (s *Server) StopWorkload(ctx context.Context, req *connect.Request[ardents.StopWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.WorkloadCommandResponse, *rpcError) {
		if err := requireWrite(call, "workload", "workload.stop"); err != nil {
			return nil, err
		}
		if err := s.workload.StopWorkloadContext(callCtx, req.Msg.GetId()); err != nil {
			return nil, mapAPIError("workload", "workload.stop", "stop_failed", "workload stop failed", true, err)
		}
		workload, err := s.workload.GetWorkloadStatus(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.stop", "failed", "workload lookup after stop failed", false, err)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload stopped", true),
			Workload: toWorkloadStatusSnapshot(workload),
		}, nil
	})
}

func (s *Server) RestartWorkload(ctx context.Context, req *connect.Request[ardents.RestartWorkloadRequest]) (*connect.Response[ardents.WorkloadCommandResponse], error) {
	callCtx, cancel := mutationContext(ctx)
	defer cancel()
	return respond(s, req.Header(), func(call callContext) (*ardents.WorkloadCommandResponse, *rpcError) {
		if err := requireWrite(call, "workload", "workload.restart"); err != nil {
			return nil, err
		}
		if err := s.workload.RestartWorkloadContext(callCtx, req.Msg.GetId()); err != nil {
			return nil, mapAPIError("workload", "workload.restart", "restart_failed", "workload restart failed", true, err)
		}
		workload, err := s.workload.GetWorkloadStatus(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.restart", "failed", "workload lookup after restart failed", false, err)
		}
		return &ardents.WorkloadCommandResponse{
			Status:   statusProto("completed", "workload restarted", true),
			Workload: toWorkloadStatusSnapshot(workload),
		}, nil
	})
}

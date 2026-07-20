package connectrpc

import (
	"context"

	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetDiagnostics(ctx context.Context, req *connect.Request[ardents.GetDiagnosticsRequest]) (*connect.Response[ardents.DiagnosticsSnapshotResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.DiagnosticsSnapshotResponse, *rpcError) {
		if err := requireRead(call, "diagnostics", "diagnostics.snapshot"); err != nil {
			return nil, err
		}
		return &ardents.DiagnosticsSnapshotResponse{
			Status:      statusProto("completed", "diagnostics snapshot available", true),
			Diagnostics: toDiagSnapshot(s.diagnostics.DiagnosticsSnapshot()),
		}, nil
	})
}

func (s *Server) GetPendingOperations(ctx context.Context, req *connect.Request[ardents.GetPendingOperationsRequest]) (*connect.Response[ardents.PendingOperationsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.PendingOperationsResponse, *rpcError) {
		if err := requireRead(call, "diagnostics", "diagnostics.pending_operations"); err != nil {
			return nil, err
		}
		return &ardents.PendingOperationsResponse{
			Status:     statusProto("completed", "pending operations available", true),
			Operations: toOperationSnapshots(s.diagnostics.PendingOperations()),
		}, nil
	})
}

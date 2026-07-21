package diagnostics

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Endpoint) GetDiagnostics(_ context.Context, req *connect.Request[ardents.GetDiagnosticsRequest]) (*connect.Response[ardents.DiagnosticsSnapshotResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.DiagnosticsSnapshotResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "diagnostics", "diagnostics.snapshot"); err != nil {
			return nil, err
		}
		return &ardents.DiagnosticsSnapshotResponse{
			Status:      operationStatus("completed", "diagnostics snapshot available", true),
			Diagnostics: toDiagSnapshot(h.service.DiagnosticsSnapshot()),
		}, nil
	})
}

func (h *Endpoint) GetPendingOperations(_ context.Context, req *connect.Request[ardents.GetPendingOperationsRequest]) (*connect.Response[ardents.PendingOperationsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.PendingOperationsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "diagnostics", "diagnostics.pending_operations"); err != nil {
			return nil, err
		}
		return &ardents.PendingOperationsResponse{
			Status:     operationStatus("completed", "pending operations available", true),
			Operations: toOperationSnapshots(h.service.PendingOperations()),
		}, nil
	})
}

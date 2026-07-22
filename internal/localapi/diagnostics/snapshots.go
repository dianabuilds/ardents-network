package diagnostics

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Endpoint) GetDiagnostics(ctx context.Context, _ *connect.Request[ardents.GetDiagnosticsRequest]) (*connect.Response[ardents.DiagnosticsSnapshotResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.DiagnosticsSnapshotResponse, *rpc.Error) {
		return &ardents.DiagnosticsSnapshotResponse{
			Status:      operationStatus("completed", "diagnostics snapshot available", true),
			Diagnostics: toDiagSnapshot(h.service.DiagnosticsSnapshot()),
		}, nil
	})
}

func (h *Endpoint) GetPendingOperations(ctx context.Context, _ *connect.Request[ardents.GetPendingOperationsRequest]) (*connect.Response[ardents.PendingOperationsResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.PendingOperationsResponse, *rpc.Error) {
		return &ardents.PendingOperationsResponse{
			Status:     operationStatus("completed", "pending operations available", true),
			Operations: toOperationSnapshots(h.service.PendingOperations()),
		}, nil
	})
}

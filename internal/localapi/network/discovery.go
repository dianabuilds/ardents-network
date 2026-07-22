package network

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) ResolveRecord(ctx context.Context, req *connect.Request[ardents.ResolveRecordRequest]) (*connect.Response[ardents.DiscoveryResult], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.DiscoveryResult, *rpc.Error) {
		res, err := h.discovery.ResolveRecord(req.Msg.GetSubject(), req.Msg.GetKind())
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.resolve_record", "failed", "discovery resolve record failed", false, err)
		}
		return toDiscoveryResult(res), nil
	})
}

func (h *API) ResolveService(ctx context.Context, req *connect.Request[ardents.ResolveServiceRequest]) (*connect.Response[ardents.ServiceResult], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.ServiceResult, *rpc.Error) {
		res, err := h.discovery.ResolveService(req.Msg.GetService())
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.resolve_service", "failed", "discovery resolve service failed", false, err)
		}
		return toServiceResult(res), nil
	})
}

func (h *API) ListRecords(ctx context.Context, _ *connect.Request[ardents.ListRecordsRequest]) (*connect.Response[ardents.ListRecordsResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.ListRecordsResponse, *rpc.Error) {
		records, err := h.records.ListRecords()
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.list_records", "failed", "discovery list records failed", false, err)
		}
		out := make([]*ardents.DiscoveryRecord, 0, len(records))
		for _, item := range records {
			out = append(out, toDiscoveryRecord(item))
		}
		return &ardents.ListRecordsResponse{
			Status:  operationStatus("completed", "records available", true),
			Records: out,
		}, nil
	})
}

func (h *API) ImportRecord(ctx context.Context, req *connect.Request[ardents.ImportRecordRequest]) (*connect.Response[ardents.RecordImportResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.RecordImportResponse, *rpc.Error) {
		res, err := h.records.ImportRecord(fromDiscoveryRecord(req.Msg.GetRecord()))
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.import", "import_failed", "discovery import failed", false, err)
		}
		return &ardents.RecordImportResponse{
			Status: operationStatus(res.State, res.Reason, res.Accepted),
		}, nil
	})
}

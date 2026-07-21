package network

import (
	"context"

	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) ResolveRecord(_ context.Context, req *connect.Request[ardents.ResolveRecordRequest]) (*connect.Response[ardents.DiscoveryResult], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.DiscoveryResult, *rpc.Error) {
		if err := rpc.RequireRead(call, "discovery", "discovery.resolve_record"); err != nil {
			return nil, err
		}
		res, err := h.discovery.ResolveRecord(req.Msg.GetSubject(), req.Msg.GetKind())
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.resolve_record", "failed", "discovery resolve record failed", false, err)
		}
		return toDiscoveryResult(res), nil
	})
}

func (h *API) ResolveService(_ context.Context, req *connect.Request[ardents.ResolveServiceRequest]) (*connect.Response[ardents.ServiceResult], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.ServiceResult, *rpc.Error) {
		if err := rpc.RequireRead(call, "discovery", "discovery.resolve_service"); err != nil {
			return nil, err
		}
		res, err := h.discovery.ResolveService(req.Msg.GetService())
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.resolve_service", "failed", "discovery resolve service failed", false, err)
		}
		return toServiceResult(res), nil
	})
}

func (h *API) ListRecords(_ context.Context, req *connect.Request[ardents.ListRecordsRequest]) (*connect.Response[ardents.ListRecordsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.ListRecordsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "discovery", "discovery.list_records"); err != nil {
			return nil, err
		}
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

func (h *API) ImportRecord(_ context.Context, req *connect.Request[ardents.ImportRecordRequest]) (*connect.Response[ardents.RecordImportResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.RecordImportResponse, *rpc.Error) {
		if err := rpc.RequireWrite(call, "discovery", "discovery.import"); err != nil {
			return nil, err
		}
		res, err := h.records.ImportRecord(fromDiscoveryRecord(req.Msg.GetRecord()))
		if err != nil {
			return nil, rpc.MapError("discovery", "discovery.import", "import_failed", "discovery import failed", false, err)
		}
		return &ardents.RecordImportResponse{
			Status: operationStatus(res.State, res.Reason, res.Accepted),
		}, nil
	})
}

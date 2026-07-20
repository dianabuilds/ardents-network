package connectrpc

import (
	"context"

	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) ResolveRecord(ctx context.Context, req *connect.Request[ardents.ResolveRecordRequest]) (*connect.Response[ardents.DiscoveryResult], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.DiscoveryResult, *rpcError) {
		if err := requireRead(call, "discovery", "discovery.resolve_record"); err != nil {
			return nil, err
		}
		res, err := s.discovery.ResolveRecord(req.Msg.GetSubject(), req.Msg.GetKind())
		if err != nil {
			return nil, mapAPIError("discovery", "discovery.resolve_record", "failed", "discovery resolve record failed", false, err)
		}
		return toDiscoveryResult(res), nil
	})
}

func (s *Server) ResolveService(ctx context.Context, req *connect.Request[ardents.ResolveServiceRequest]) (*connect.Response[ardents.ServiceResult], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.ServiceResult, *rpcError) {
		if err := requireRead(call, "discovery", "discovery.resolve_service"); err != nil {
			return nil, err
		}
		res, err := s.discovery.ResolveService(req.Msg.GetService())
		if err != nil {
			return nil, mapAPIError("discovery", "discovery.resolve_service", "failed", "discovery resolve service failed", false, err)
		}
		return toServiceResult(res), nil
	})
}

func (s *Server) ListRecords(ctx context.Context, req *connect.Request[ardents.ListRecordsRequest]) (*connect.Response[ardents.ListRecordsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.ListRecordsResponse, *rpcError) {
		if err := requireRead(call, "discovery", "discovery.list_records"); err != nil {
			return nil, err
		}
		records, err := s.discovery.ListRecords()
		if err != nil {
			return nil, mapAPIError("discovery", "discovery.list_records", "failed", "discovery list records failed", false, err)
		}
		out := make([]*ardents.DiscoveryRecord, 0, len(records))
		for _, item := range records {
			out = append(out, toDiscoveryRecord(item))
		}
		return &ardents.ListRecordsResponse{
			Status:  statusProto("completed", "records available", true),
			Records: out,
		}, nil
	})
}

func (s *Server) ImportRecord(ctx context.Context, req *connect.Request[ardents.ImportRecordRequest]) (*connect.Response[ardents.RecordImportResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.RecordImportResponse, *rpcError) {
		if err := requireWrite(call, "discovery", "discovery.import"); err != nil {
			return nil, err
		}
		res, err := s.discovery.ImportRecord(fromDiscoveryRecord(req.Msg.GetRecord()))
		if err != nil {
			return nil, mapAPIError("discovery", "discovery.import", "import_failed", "discovery import failed", false, err)
		}
		return &ardents.RecordImportResponse{
			Status: statusProto(res.State, res.Reason, res.Accepted),
		}, nil
	})
}

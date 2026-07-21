package transfer

import (
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Handler) FetchBlob(ctx context.Context, req *connect.Request[ardentsv1.FetchBlobRequest]) (*connect.Response[ardentsv1.BlobSnapshot], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.BlobSnapshot, *rpc.Error) {
		if err := rpc.RequireWrite(call, "data", "data.fetch_blob"); err != nil {
			return nil, err
		}
		res, err := h.fetcher.FetchBlob(ctx, req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("data", "data.fetch_blob", "fetch_failed", "data fetch blob failed", true, err)
		}
		return blobSnapshot(res), nil
	})
}

func (h *Handler) ListBlobSources(_ context.Context, req *connect.Request[ardentsv1.ListBlobSourcesRequest]) (*connect.Response[ardentsv1.ListBlobSourcesResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListBlobSourcesResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.blob_sources"); err != nil {
			return nil, err
		}
		return &ardentsv1.ListBlobSourcesResponse{Sources: sourceSnapshots(h.sources.ListBlobSources(req.Msg.GetId()))}, nil
	})
}

func (h *Handler) GetTransfer(_ context.Context, req *connect.Request[ardentsv1.GetTransferRequest]) (*connect.Response[ardentsv1.GetTransferResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.GetTransferResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.get_transfer"); err != nil {
			return nil, err
		}
		res, ok := h.records.Get(req.Msg.GetId())
		if !ok {
			return nil, rpc.NotFound("data", "data.get_transfer", "data transfer not found")
		}
		return &ardentsv1.GetTransferResponse{Status: &ardentsv1.OperationStatus{State: "completed", Reason: "transfer available", Accepted: true}, Transfer: transferSnapshot(res)}, nil
	})
}

func (h *Handler) ListTransfers(_ context.Context, req *connect.Request[ardentsv1.ListTransfersRequest]) (*connect.Response[ardentsv1.ListTransfersResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListTransfersResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "data", "data.list_transfers"); err != nil {
			return nil, err
		}
		return &ardentsv1.ListTransfersResponse{Transfers: transferSnapshots(h.records.List())}, nil
	})
}

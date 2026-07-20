package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetTransfer(ctx context.Context, req *connect.Request[ardentsv1.GetTransferRequest]) (*connect.Response[ardentsv1.GetTransferResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.GetTransferResponse, *rpcError) {
		if err := requireRead(call, "data", "data.get_transfer"); err != nil {
			return nil, err
		}
		res, err := s.data.GetTransfer(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("data", "data.get_transfer", "failed", "data get transfer failed", false, err)
		}
		return &ardentsv1.GetTransferResponse{
			Status:   statusProto("completed", "transfer available", true),
			Transfer: toTransferSnapshot(res),
		}, nil
	})
}

func (s *Server) ListTransfers(ctx context.Context, req *connect.Request[ardentsv1.ListTransfersRequest]) (*connect.Response[ardentsv1.ListTransfersResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListTransfersResponse, *rpcError) {
		if err := requireRead(call, "data", "data.list_transfers"); err != nil {
			return nil, err
		}
		return &ardentsv1.ListTransfersResponse{Transfers: toTransferSnapshots(s.data.ListTransfers())}, nil
	})
}

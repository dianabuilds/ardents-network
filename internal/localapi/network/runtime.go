package network

import (
	"context"

	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) GetNetworkStatus(_ context.Context, req *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*protocol.NetworkStatusResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "transport", "transport.network_status"); err != nil {
			return nil, err
		}
		return &protocol.NetworkStatusResponse{Status: operationStatus("completed", "network status available", true), Network: networkStatus(h.status.GetNetworkStatus())}, nil
	})
}

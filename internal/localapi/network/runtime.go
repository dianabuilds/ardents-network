package network

import (
	"context"

	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) GetNetworkStatus(ctx context.Context, _ *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*protocol.NetworkStatusResponse, *rpc.Error) {
		return &protocol.NetworkStatusResponse{Status: operationStatus("completed", "network status available", true), Network: networkStatus(h.status.GetNetworkStatus())}, nil
	})
}

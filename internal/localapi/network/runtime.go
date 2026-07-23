package network

import (
	"context"

	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *API) GetNetworkStatus(ctx context.Context, _ *connect.Request[protocol.GetNetworkStatusRequest]) (*connect.Response[protocol.NetworkStatusResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*protocol.NetworkStatusResponse, *rpc.Error) {
		snapshot, err := networkStatus(h.status.GetNetworkStatus())
		if err != nil {
			return nil, rpc.MapError("transport", "transport.network_status", "failed", "network status is invalid", false, err)
		}
		return &protocol.NetworkStatusResponse{Status: operationStatus("completed", "network status available", true), Network: snapshot}, nil
	})
}

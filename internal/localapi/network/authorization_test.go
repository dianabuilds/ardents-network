package network

import (
	"context"
	"testing"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestHandlerRequiresInterceptorCallContext(t *testing.T) {
	handler := NewHandler(nil, nil, nil)
	request := connect.NewRequest(&protocol.GetNetworkStatusRequest{})
	request.Header().Set("Authorization", "Bearer must-not-be-parsed-here")
	_, err := handler.GetNetworkStatus(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

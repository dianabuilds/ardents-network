package transfer

import (
	"context"
	"testing"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestHandlerRequiresInterceptorCallContext(t *testing.T) {
	handler := NewHandler(nil, nil, nil)
	request := connect.NewRequest(&protocol.ListTransfersRequest{})
	request.Header().Set("Authorization", "Bearer must-not-be-parsed-here")
	_, err := handler.ListTransfers(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

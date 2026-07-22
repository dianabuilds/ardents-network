package content

import (
	"context"
	"testing"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestHandlerRequiresInterceptorCallContext(t *testing.T) {
	handler, err := NewHandler(inventoryQueryStub{}, nil)
	require.NoError(t, err)
	request := connect.NewRequest(&protocol.GetDataInventoryRequest{})
	request.Header().Set("Authorization", "Bearer must-not-be-parsed-here")
	_, err = handler.GetDataInventory(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

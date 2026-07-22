package diagnostics

import (
	"context"
	"testing"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestHandlerRequiresInterceptorCallContext(t *testing.T) {
	handler := NewHandler(nil)
	request := connect.NewRequest(&protocol.GetDiagnosticsRequest{})
	request.Header().Set("Authorization", "Bearer must-not-be-parsed-here")
	_, err := handler.GetDiagnostics(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

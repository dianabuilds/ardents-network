package rpc

import (
	"context"
	"errors"
	"testing"

	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestRespondRequiresInterceptorCallContextAndPreservesErrorMapping(t *testing.T) {
	_, err := RespondContext(context.Background(), func(Call) (*protocol.CommandAckResponse, *Error) {
		require.FailNow(t, "invoke should not run without auth")
		return nil, nil
	})
	connectErr, ok := errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())

	connectErr, ok = errors.AsType[*connect.Error](ToConnectError(&Error{Category: "not_found", Message: "missing resource"}))
	require.True(t, ok)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

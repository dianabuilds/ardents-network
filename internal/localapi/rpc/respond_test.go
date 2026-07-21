package rpc

import (
	"errors"
	"net/http"
	"testing"

	"ardents/internal/identity"
	localauth "ardents/internal/localapi/auth"
	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestRespondPreservesAuthAndErrorMapping(t *testing.T) {
	auth := localauth.Config{Token: "token", SubjectID: "rpc", Capabilities: []string{"node.status"}}
	header := http.Header{"Authorization": []string{"Bearer token"}}

	_, err := Respond(auth, header, func(call CallContext) (*protocol.CommandAckResponse, *Error) {
		require.True(t, call.Authenticated)
		require.Equal(t, identity.SubjectRef{Kind: "token", ID: "rpc"}, call.Subject)
		return nil, &Error{Category: "not_found", Message: "missing resource"}
	})
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())

	_, err = Respond(auth, http.Header{}, func(CallContext) (*protocol.CommandAckResponse, *Error) {
		require.FailNow(t, "invoke should not run without auth")
		return nil, nil
	})
	connectErr, ok := errors.AsType[*connect.Error](err)
	require.True(t, ok)
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

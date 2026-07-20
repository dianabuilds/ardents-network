package connectrpc

import (
	"errors"
	"net/http"
	"testing"

	identityapi "ardents/internal/identity/api"
	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestRespondPreservesAuthAndConnectErrorMapping(t *testing.T) {
	server := &Server{auth: AuthConfig{Token: "token", SubjectID: "rpc", Capabilities: []string{"node.status"}}}

	header := http.Header{}
	header.Set("Authorization", "Bearer token")
	_, err := respond(server, header, func(call callContext) (*ardentsv1.CommandAckResponse, *rpcError) {
		require.True(t, call.Authenticated)
		require.Equal(t, identityapi.SubjectRef{Kind: "token", ID: "rpc"}, call.Subject)
		require.Equal(t, []string{"node.status"}, call.Capabilities)
		return nil, &rpcError{Category: "not_found", Message: "missing resource"}
	})
	require.Error(t, err, "expected mapped connect error")

	var connectErr *connect.Error
	require.Truef(t, errors.As(err, &connectErr), "err = %T, want *connect.Error", err)
	require.Falsef(t, connectErr.Code() !=

		connect.
			CodeNotFound, "code = %v, want not_found", connectErr.Code())

	_, err = respond(server, http.Header{}, func(callContext) (*ardentsv1.CommandAckResponse, *rpcError) {
		require.FailNow(t, "invoke should not run without auth")
		return nil, nil
	})
	require.Error(t, err, "expected unauthenticated error")
	require.Truef(t, errors.As(err, &connectErr), "err = %T, want *connect.Error", err)
	require.Falsef(t, connectErr.Code() !=

		connect.
			CodeUnauthenticated, "code = %v, want unauthenticated", connectErr.Code())

}

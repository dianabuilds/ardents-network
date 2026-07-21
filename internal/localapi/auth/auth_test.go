package auth

import (
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestConfigValidateRequiresTokenAndCapabilities(t *testing.T) {
	{
		err := (Config{}).validate()
		require.Error(t, err, "expected missing token/scopes error")
	}
	{

		err := (Config{Token: "token"}).validate()
		require.Error(t, err, "expected missing scopes error")
	}
	{

		err := (Config{Token: "token", Capabilities: []string{"*"}}).validate()
		require.NoErrorf(t, err, "unexpected validate error: %v", err)
	}

}

func TestConfigCallContextValidatesAuthorizationHeader(t *testing.T) {
	cfg := Config{Token: "token", SubjectID: "rpc", Capabilities: []string{"node.status"}}
	header := http.Header{}
	header.Set("Authorization", "Bearer token")

	call, err := cfg.CallContext(header)
	require.NoErrorf(t, err, "callContext: %v", err)
	require.True(t, call.Authenticated)
	require.Equal(t, "token", call.Subject.Kind)
	require.Equal(t, "rpc", call.Subject.ID)
	require.Equal(t, []string{"node.status"}, call.Capabilities)

	_, err = cfg.CallContext(http.Header{})
	connectErr, ok := errors.AsType[*connect.Error](err)
	require.Truef(t, ok, "error = %T, want *connect.Error", err)
	require.Falsef(t, connectErr.Code() !=

		connect.
			CodeUnauthenticated, "code = %v, want unauthenticated", connectErr.Code())

}

func TestConfigRejectsCredentialForDifferentLocalPrincipal(t *testing.T) {
	cfg := Config{Token: "principal-a-token", SubjectID: "principal-a", Capabilities: []string{"node.status"}}
	header := http.Header{}
	header.Set("Authorization", "Bearer principal-b-token")

	_, err := cfg.CallContext(header)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	require.NotContains(t, connectErr.Message(), "principal-a-token")
	require.NotContains(t, connectErr.Message(), "principal-b-token")
}

func TestConfigCallContextRequiresStrictBearerScheme(t *testing.T) {
	cfg := Config{Token: "token", Capabilities: []string{"node.status"}}
	for _, headerValue := range []string{
		"token",
		"bearer token",
		"Token token",
		"Bearer",
		"Bearer token extra",
	} {
		header := http.Header{}
		header.Set("Authorization", headerValue)
		{
			_, err := cfg.CallContext(header)
			require.Errorf(t, err, "expected unauthenticated error for header %q", headerValue)
		}

	}
}

func TestConfigCallContextDefaultsSubjectIDWhenNotConfigured(t *testing.T) {
	cfg := Config{Token: "token", Capabilities: []string{"node.status"}}
	header := http.Header{}
	header.Set("Authorization", "Bearer token")

	call, err := cfg.CallContext(header)
	require.NoError(t, err)
	require.Equal(t, "local-api", call.Subject.ID)
	require.Equal(t, "token", call.Subject.Kind)
	require.Equal(t, []string{"node.status"}, call.Capabilities)
}

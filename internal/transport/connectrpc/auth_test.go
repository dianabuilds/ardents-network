package connectrpc

import (
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestAuthConfigValidateRequiresTokenAndCapabilities(t *testing.T) {
	{
		err := (AuthConfig{}).validate()
		require.Error(t, err, "expected missing token/scopes error")
	}
	{

		err := (AuthConfig{Token: "token"}).validate()
		require.Error(t, err, "expected missing scopes error")
	}
	{

		err := (AuthConfig{Token: "token", Capabilities: []string{"*"}}).validate()
		require.NoErrorf(t, err, "unexpected validate error: %v", err)
	}

}

func TestAuthConfigCallContextValidatesAuthorizationHeader(t *testing.T) {
	cfg := AuthConfig{Token: "token", SubjectID: "rpc", Capabilities: []string{"node.status"}}
	header := http.Header{}
	header.Set("Authorization", "Bearer token")

	call, err := cfg.callContext(header)
	require.NoErrorf(t, err, "callContext: %v", err)
	require.True(t, call.Authenticated)
	require.Equal(t, "token", call.Subject.Kind)
	require.Equal(t, "rpc", call.Subject.ID)
	require.Equal(t, []string{"node.status"}, call.Capabilities)

	_, err = cfg.callContext(http.Header{})
	var connectErr *connect.Error
	require.Truef(t, errors.As(err, &connectErr), "error = %T, want *connect.Error", err)
	require.Falsef(t, connectErr.Code() !=

		connect.
			CodeUnauthenticated, "code = %v, want unauthenticated", connectErr.Code())

}

func TestAuthConfigRejectsCredentialForDifferentLocalPrincipal(t *testing.T) {
	cfg := AuthConfig{Token: "principal-a-token", SubjectID: "principal-a", Capabilities: []string{"node.status"}}
	header := http.Header{}
	header.Set("Authorization", "Bearer principal-b-token")

	_, err := cfg.callContext(header)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	require.NotContains(t, connectErr.Message(), "principal-a-token")
	require.NotContains(t, connectErr.Message(), "principal-b-token")
}

func TestAuthConfigCallContextRequiresStrictBearerScheme(t *testing.T) {
	cfg := AuthConfig{Token: "token", Capabilities: []string{"node.status"}}
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
			_, err := cfg.callContext(header)
			require.Errorf(t, err, "expected unauthenticated error for header %q", headerValue)
		}

	}
}

func TestAuthConfigCallContextDefaultsSubjectIDWhenNotConfigured(t *testing.T) {
	cfg := AuthConfig{Token: "token", Capabilities: []string{"node.status"}}
	header := http.Header{}
	header.Set("Authorization", "Bearer token")

	call, err := cfg.callContext(header)
	require.NoError(t, err)
	require.Equal(t, "local-api", call.Subject.ID)
	require.Equal(t, "token", call.Subject.Kind)
	require.Equal(t, []string{"node.status"}, call.Capabilities)
}

package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

type commandSessionClient struct {
	key       client.SessionKey
	logoutErr error
}

func (c *commandSessionClient) Login(context.Context) (client.SessionKey, error) { return c.key, nil }
func (c *commandSessionClient) SessionStatus() client.SessionKey                 { return c.key }
func (c *commandSessionClient) Logout() error {
	c.key = client.SessionKey{}
	return c.logoutErr
}

func TestLogoutReportsUnconfirmedServerInvalidationAfterLocalCleanup(t *testing.T) {
	key := client.SessionKey{NodePrincipal: "p1_alpha", SignerPrincipal: "p1_alice", ProtocolMajor: 1}
	sessions := &commandSessionClient{key: key, logoutErr: connect.NewError(connect.CodeUnavailable, errors.New("server offline"))}
	var stdout, stderr bytes.Buffer
	command := NewOnline(output.NewRenderer(&stdout, &stderr, true), sessions, time.Second, bytes.NewReader(nil))

	require.Equal(t, 1, command.Run(context.Background(), []string{"logout"}))
	require.Equal(t, client.SessionKey{}, sessions.key)
	require.Empty(t, stdout.String())
	var failure map[string]any
	require.NoError(t, json.Unmarshal(stderr.Bytes(), &failure))
	require.Equal(t, "unavailable", failure["code"])
	require.Contains(t, failure["message"], "local Session cleared")
	require.Contains(t, failure["message"], "server invalidation unconfirmed")
	require.Contains(t, failure["message"], "server offline")
}

func (c *commandSessionClient) PublicIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	return nil, nil
}
func (c *commandSessionClient) ProtectedIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	return nil, nil
}
func (c *commandSessionClient) TargetNodePrincipal() string { return c.key.NodePrincipal }

func TestSessionCommandsExposeOnlyPublicCacheMetadata(t *testing.T) {
	key := client.SessionKey{NodePrincipal: "p1_alpha", SignerPrincipal: "p1_alice", ProtocolMajor: 1}
	sessions := &commandSessionClient{key: key}
	var stdout, stderr bytes.Buffer
	command := NewOnline(output.NewRenderer(&stdout, &stderr, true), sessions, time.Second, bytes.NewReader(nil))

	require.Zero(t, command.Run(context.Background(), []string{"login"}))
	require.JSONEq(t, `{"status":"authenticated","node_principal":"p1_alpha","signer_principal":"p1_alice","interface":"operator","protocol_major":1}`, stdout.String())
	require.NotContains(t, stdout.String(), "session_secret")
	require.Empty(t, stderr.String())

	stdout.Reset()
	require.Zero(t, command.Run(context.Background(), []string{"logout"}))
	require.JSONEq(t, `{"status":"not_authenticated"}`, stdout.String())
	require.Equal(t, client.SessionKey{}, sessions.key)
}

package identity

import (
	"bytes"
	"context"
	"testing"
	"time"

	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
)

type commandSessionClient struct {
	key client.SessionKey
}

func (c *commandSessionClient) Login(context.Context) (client.SessionKey, error) { return c.key, nil }
func (c *commandSessionClient) SessionStatus() client.SessionKey                 { return c.key }
func (c *commandSessionClient) Logout()                                          { c.key = client.SessionKey{} }
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

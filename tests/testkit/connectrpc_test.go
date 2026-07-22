package testkit

import (
	"context"
	"testing"

	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"

	"github.com/stretchr/testify/require"
)

func TestOperatorPrincipalAccessFixtureAdmitsSession(t *testing.T) {
	service, node, session, peer, _ := newOperatorPrincipalAccess(t)
	resource, err := identityaccess.NewResourceRef(node, "", "node", "")
	require.NoError(t, err)
	call, err := service.Admit(context.Background(), identityaccess.Attempt{
		SessionSecret: session,
		Binding: identityaccess.AuthenticationBinding{
			Audience: identityaccess.Audience{
				Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1,
			},
			TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
			PeerBinding:      peer,
		},
		Action: "node.status", Resource: resource,
	})
	require.NoError(t, err)
	require.True(t, call.IsAdmitted())
	require.NotEmpty(t, call.Actor())
	require.Equal(t, call.Actor(), call.Effective())
}

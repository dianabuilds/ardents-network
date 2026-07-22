package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/storage"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestBeginAuthenticationDerivesOperatorUnixBinding(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	service, err := identityaccess.NewService(identityaccess.Config{Database: database})
	require.NoError(t, err)
	_, nodeKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	_, principalKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(principalKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	var peer [32]byte
	peer[0] = 7
	var source identityaccess.SourceKey
	source[0] = 9
	h := &Handler{service: service, node: node.String(), fallback: transportPeer{peer: peer, source: source}}

	response, err := h.BeginAuthentication(context.Background(), connect.NewRequest(&protocol.BeginAuthenticationRequest{PrincipalId: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION}))
	require.NoError(t, err)
	require.Equal(t, node.String(), response.Msg.Challenge.Binding.Audience.Node)
	require.Equal(t, identityprotocol.Interface_INTERFACE_OPERATOR, response.Msg.Challenge.Binding.Audience.Interface)
	require.Equal(t, identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, response.Msg.Challenge.Binding.TransportProfile)
	require.Equal(t, peer[:], response.Msg.Challenge.Binding.PeerBinding)
}

func TestTransportPeerContextOverridesSharedFallback(t *testing.T) {
	h := &Handler{node: "node", fallback: transportPeer{peer: [32]byte{1}, source: identityaccess.SourceKey{2}}}
	fallbackBinding, fallbackSource := h.binding(context.Background())
	peer := [32]byte{3}
	source := identityaccess.SourceKey{4}
	peerBinding, peerSource := h.binding(identityaccess.WithTransportPeer(context.Background(), peer, source))
	require.Equal(t, byte(1), fallbackBinding.PeerBinding[0])
	require.Equal(t, byte(2), fallbackSource[0])
	require.Equal(t, peer, peerBinding.PeerBinding)
	require.Equal(t, source, peerSource)
	require.NotEqual(t, fallbackBinding.PeerBinding, peerBinding.PeerBinding)
}

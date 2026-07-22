package daemon

import (
	"context"
	"testing"

	identityaccess "ardents/internal/identity/access"

	"github.com/stretchr/testify/require"
)

func TestCanonicalUnixPeerUIDIsStableAndDistinct(t *testing.T) {
	require.Equal(t, canonicalUnixPeerUID(1000), canonicalUnixPeerUID(1000))
	require.NotEqual(t, canonicalUnixPeerUID(1000), canonicalUnixPeerUID(1001))
	require.Equal(t, []byte{1, 0, 0, 3, 232}, canonicalUnixPeerUID(1000))
}

func TestOperatorAndApplicationUnixBindingsAreDomainSeparated(t *testing.T) {
	operator := operatorTransportContext(context.Background(), nil, "/run/ardents/shared.sock")
	application := applicationTransportContext(context.Background(), nil, "/run/ardents/shared.sock")
	operatorPeer, operatorSource, ok := identityaccess.TransportPeerFromContext(operator)
	require.True(t, ok)
	applicationPeer, applicationSource, ok := identityaccess.TransportPeerFromContext(application)
	require.True(t, ok)
	require.NotZero(t, operatorPeer)
	require.NotZero(t, applicationPeer)
	require.NotEqual(t, operatorPeer, applicationPeer)
	require.NotEqual(t, operatorSource, applicationSource)
}

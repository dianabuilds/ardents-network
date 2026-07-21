package provision

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"

	"github.com/stretchr/testify/require"
)

func TestOperatorDocumentEnablesRealPrivateChannels(t *testing.T) {
	public := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	doc := operatorDocument(options{nodeName: "peer2", transportPort: 61002, bootstrapPeer: "/dns4/seed/tcp/61001/p2p/peer"},
		NodeProvision{Subject: "p_subject", Issuer: "p_issuer", IssuerPublic: public,
			DiscoveryRef: "discovery-ref", DataRef: "data-ref", ApplicationExpiresAt: time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)})

	require.NoError(t, runtimeconfig.Validate(doc))
	require.True(t, doc.Privacy.Required)
	require.Equal(t, []string{"/dns4/seed/tcp/61001/p2p/peer"}, doc.Network.BootstrapPeers)
	require.NotEqual(t, doc.Privacy.Discovery.Reference, doc.Privacy.Data.Reference)
	require.True(t, doc.ApplicationInterface.Enabled)
	require.Equal(t, "127.0.0.1:8081", doc.ApplicationInterface.ListenAddress)
	require.Equal(t, filepath.Join("/var/lib/ardents", "applications", "application.sock"), doc.ApplicationInterface.SocketPath)
	require.Equal(t, filepath.Join("/var/lib/ardents", "applications", "application-token"), doc.ApplicationInterface.TokenFile)
	require.Equal(t, []string{"application.content.put", "application.content.get"}, doc.ApplicationInterface.Capabilities)
	require.Equal(t, "2026-08-21T00:00:00Z", doc.ApplicationInterface.CredentialExpiresAt)
}

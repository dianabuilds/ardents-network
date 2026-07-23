package provision

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"

	runtimeconfig "ardents/internal/config"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TestOperatorDocumentEnablesRealPrivateChannels(t *testing.T) {
	public := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	issuer, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	doc := operatorDocument(options{nodeName: "peer2", transportPort: 61002, bootstrapPeer: "/dns4/seed/tcp/61001/p2p/peer"},
		NodeProvision{Subject: "p_subject", Issuer: issuer.String(), IssuerPublic: public,
			DiscoveryRef: "discovery-ref", DataRef: "data-ref"})

	require.NoError(t, runtimeconfig.Validate(doc))
	require.True(t, doc.Privacy.Required)
	require.Equal(t, []string{"/dns4/seed/tcp/61001/p2p/peer"}, doc.Network.BootstrapPeers)
	require.NotEqual(t, doc.Privacy.Discovery.Reference, doc.Privacy.Data.Reference)
	require.True(t, doc.ApplicationInterface.Enabled)
	require.Equal(t, filepath.Join("/var/lib/ardents-applications", "application.sock"), doc.ApplicationInterface.SocketPath)
	require.Equal(t, []runtimeconfig.TrustedPrincipalConfig{{
		Principal: issuer.String(), PublicKey: doc.Trust.Principals[0].PublicKey,
		Purposes: []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}}, doc.Trust.Principals)
}

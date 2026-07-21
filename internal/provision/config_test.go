package provision

import (
	"crypto/ed25519"
	"testing"

	runtimeconfig "ardents/internal/config"

	"github.com/stretchr/testify/require"
)

func TestOperatorDocumentEnablesRealPrivateChannels(t *testing.T) {
	public := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	doc := operatorDocument(options{nodeName: "peer2", transportPort: 61002, bootstrapPeer: "/dns4/seed/tcp/61001/p2p/peer"},
		NodeProvision{Subject: "p_subject", Issuer: "p_issuer", IssuerPublic: public,
			DiscoveryRef: "discovery-ref", DataRef: "data-ref"})

	require.NoError(t, runtimeconfig.Validate(doc))
	require.True(t, doc.Privacy.Required)
	require.Equal(t, []string{"/dns4/seed/tcp/61001/p2p/peer"}, doc.Network.BootstrapPeers)
	require.NotEqual(t, doc.Privacy.Discovery.Reference, doc.Privacy.Data.Reference)
}

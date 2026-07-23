package testkit

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"

	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func DiscoveryTrustRegistry(t *testing.T, encodedKeys ...string) *identitytrust.Registry {
	t.Helper()
	entries := make([]identitytrust.Entry, 0, len(encodedKeys))
	for _, encoded := range encodedKeys {
		public, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		principalID, err := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(public))
		require.NoError(t, err)
		entries = append(entries, identitytrust.Entry{
			Principal: principalID.String(), PublicKey: ed25519.PublicKey(public),
			Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
		})
	}
	registry, err := identitytrust.NewRegistry(entries)
	require.NoError(t, err)
	return registry
}

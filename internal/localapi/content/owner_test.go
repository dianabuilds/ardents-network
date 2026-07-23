package content

import (
	"ardents/internal/content"
	identityprincipal "ardents/internal/identity/principal"
	protocol "ardents/internal/localapi/protocol"
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperatorWireOwnerIsNeverImportedAsAuthority(t *testing.T) {
	object := fromObjectSnapshot(&protocol.ObjectSnapshot{Id: "object", Owner: "claimed-owner"})
	manifest := fromManifestSnapshot(&protocol.ManifestSnapshot{Id: "manifest", Owner: "claimed-owner"})
	require.Empty(t, object.Owner.String())
	require.Empty(t, manifest.Owner.String())
}

func TestTypedContentOwnerProjectsAsCanonicalPrincipal(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = 0x61
	}
	owner, err := identityprincipal.FromEd25519PublicKey(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
	require.NoError(t, err)
	require.Equal(t, owner.String(), toObjectSnapshot(content.Object{Owner: owner}).GetOwner())
	require.Equal(t, owner.String(), toManifestSnapshot(content.Manifest{Owner: owner}).GetOwner())
}

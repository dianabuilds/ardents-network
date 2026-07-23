package daemon

import (
	identityapi "ardents/internal/identity"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadStartupStateWrapsStepFailure(t *testing.T) {
	err := LoadStartupState(
		func() error { return nil },
		func() error { return errors.New("discovery failed") },
		func() error { return nil },
		func() error { return nil },
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "load discovery")
}

func TestInitializeIdentityForStartupPublishesIdentityOutputs(t *testing.T) {
	var privateSet bool
	var localNodeID string
	var trustedPrincipal, trustedKey string
	var discoveryLoaded bool
	var synced bool

	err := InitializeIdentityForStartup(
		func() (identityapi.Summary, ed25519.PrivateKey, error) {
			return identityapi.Summary{
				Principal: "node-1",
				PublicKey: "pub-1",
			}, ed25519.PrivateKey("private"), nil
		},
		func(ed25519.PrivateKey) { privateSet = true },
		func(nodeID string) error { localNodeID = nodeID; return nil },
		func(principal, key string) error { trustedPrincipal, trustedKey = principal, key; return nil },
		func() error {
			require.Equal(t, "node-1", trustedPrincipal)
			discoveryLoaded = true
			return nil
		},
		func() { synced = true },
	)

	require.NoError(t, err)
	require.True(t, privateSet)
	require.Equal(t, "node-1", localNodeID)
	require.Equal(t, "node-1", trustedPrincipal)
	require.Equal(t, "pub-1", trustedKey)
	require.True(t, discoveryLoaded)
	require.True(t, synced)
}

func TestInitializeIdentityForStartupStopsWhenLocalPrincipalPropagationFails(t *testing.T) {
	discoveryLoaded := false
	err := InitializeIdentityForStartup(
		func() (identityapi.Summary, ed25519.PrivateKey, error) {
			return identityapi.Summary{Principal: "invalid", PublicKey: "public"}, ed25519.PrivateKey("private"), nil
		},
		func(ed25519.PrivateKey) {},
		func(string) error { return errors.New("invalid local Node Principal") },
		func(string, string) error { return nil },
		func() error { discoveryLoaded = true; return nil },
		func() {},
	)
	require.ErrorContains(t, err, "invalid local Node Principal")
	require.False(t, discoveryLoaded)
}

package recovery_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	identityapi "ardents/internal/identity/api"
	noderecovery "ardents/internal/node/recovery"

	"github.com/stretchr/testify/require"
)

func TestLoadStartupStateWrapsStepFailure(t *testing.T) {
	err := noderecovery.LoadStartupState(
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
	var trustedKey string
	var synced bool

	err := noderecovery.InitializeIdentityForStartup(
		func() (identityapi.Summary, ed25519.PrivateKey, error) {
			return identityapi.Summary{
				Principal: "node-1",
				PublicKey: "pub-1",
			}, ed25519.PrivateKey("private"), nil
		},
		func(ed25519.PrivateKey) { privateSet = true },
		func(nodeID string) { localNodeID = nodeID },
		func(key string) { trustedKey = key },
		func() { synced = true },
	)

	require.NoError(t, err)
	require.True(t, privateSet)
	require.Equal(t, "node-1", localNodeID)
	require.Equal(t, "pub-1", trustedKey)
	require.True(t, synced)
}

package provision

import (
	identityapi "ardents/internal/identity"
	apppolicy "ardents/internal/policy"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthorityProvisionsDistinctNodesIdempotently(t *testing.T) {
	root := t.TempDir()
	authority, err := OpenOrCreate(filepath.Join(root, "authority"))
	require.NoError(t, err)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	admission := apppolicy.New(apppolicy.Config{})

	first, err := authority.ProvisionNode(NodeOptions{
		DataDir: filepath.Join(root, "node-a"), SecretDir: filepath.Join(root, "secret-a"),
		Clock: func() time.Time { return now },
	}, admission)
	require.NoError(t, err)
	again, err := authority.ProvisionNode(NodeOptions{
		DataDir: filepath.Join(root, "node-a"), SecretDir: filepath.Join(root, "secret-a"),
		Clock: func() time.Time { return now.Add(time.Hour) },
	}, admission)
	require.NoError(t, err)
	second, err := authority.ProvisionNode(NodeOptions{
		DataDir: filepath.Join(root, "node-b"), SecretDir: filepath.Join(root, "secret-b"),
		Clock: func() time.Time { return now },
	}, admission)
	require.NoError(t, err)
	synced, err := authority.ProvisionNode(NodeOptions{
		DataDir: filepath.Join(root, "node-a"), SecretDir: filepath.Join(root, "secret-a"),
		Clock: func() time.Time { return now.Add(2 * time.Hour) },
	}, admission)
	require.NoError(t, err)

	require.Equal(t, first.Subject, again.Subject)
	require.Equal(t, first.DiscoveryRef, again.DiscoveryRef)
	require.Equal(t, first.DataRef, again.DataRef)
	require.Equal(t, first.DiscoveryRef, synced.DiscoveryRef)
	require.NotEqual(t, first.Subject, second.Subject)
	require.NotEqual(t, first.DiscoveryRef, second.DiscoveryRef)
	require.Equal(t, first.Issuer, second.Issuer)
	require.NotEqual(t, first.StoreKeyPath, second.StoreKeyPath)
	require.Equal(t, "channel-grant-store.key", filepath.Base(first.StoreKeyPath))
	require.Equal(t, "channel-grants.db", filepath.Base(first.ChannelGrantStore))
	require.NotEqual(t, first.ReplayKeyPath, second.ReplayKeyPath)
	require.NotEqual(t, identityapi.CapabilityRef(""), first.DiscoveryRef)
}

package daemon

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOwnersWiresRuntimeAndDataInventory(t *testing.T) {
	owners := NewOwners(Config{
		Name: "app-build-test",
		Data: DataConfig{Dir: t.TempDir()},
	})
	require.NotNil(t, owners.Node)
	require.Zero(t, owners.Content.InventorySnapshot().Blobs)
}

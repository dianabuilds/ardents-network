package content

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInventorySnapshotReturnsEmptyAuthoritativeTruth(t *testing.T) {
	service := NewInDir(t.TempDir())

	require.Equal(t, InventorySnapshot{}, service.InventorySnapshot())
}

package content

import (
	"testing"

	"ardents/internal/content"
	"github.com/stretchr/testify/require"
)

type inventoryQueryStub struct {
	Reader
	snapshot content.InventorySnapshot
}

func (s inventoryQueryStub) InventorySnapshot() content.InventorySnapshot {
	return s.snapshot
}

func TestDataInventoryHandlerMapsContentSnapshot(t *testing.T) {
	handler, err := NewHandler(inventoryQueryStub{snapshot: content.InventorySnapshot{
		Objects: 2, Blobs: 3, LocalBytes: 512,
	}}, nil)
	require.NoError(t, err)

	response := handler.dataInventory()
	require.EqualValues(t, 2, response.GetObjects())
	require.EqualValues(t, 3, response.GetBlobs())
	require.EqualValues(t, 512, response.GetLocalBytes())
}

func TestDataInventoryHandlerRequiresQuery(t *testing.T) {
	handler, err := NewHandler(nil, nil)

	require.ErrorIs(t, err, ErrQueryRequired)
	require.Nil(t, handler)
}

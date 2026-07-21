package content

import (
	"context"
	"testing"

	"ardents/internal/content"
	localauth "ardents/internal/localapi/auth"
	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
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
	}}, nil, localauth.Config{})
	require.NoError(t, err)

	response, err := handler.GetDataInventory(
		context.Background(), connect.NewRequest(&ardentsv1.GetDataInventoryRequest{}),
	)
	require.NoError(t, err)
	require.EqualValues(t, 2, response.Msg.GetObjects())
	require.EqualValues(t, 3, response.Msg.GetBlobs())
	require.EqualValues(t, 512, response.Msg.GetLocalBytes())
}

func TestDataInventoryHandlerRequiresQuery(t *testing.T) {
	handler, err := NewHandler(nil, nil, localauth.Config{})

	require.ErrorIs(t, err, ErrQueryRequired)
	require.Nil(t, handler)
}

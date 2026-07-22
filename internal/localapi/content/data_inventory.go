// Package content owns content and transfer protocol handlers and mappings.
// It does not own content or transfer ownership.
package content

import (
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
	"context"

	"connectrpc.com/connect"
)

func (h *QueryHandler) GetDataInventory(
	ctx context.Context,
	_ *connect.Request[ardentsv1.GetDataInventoryRequest],
) (*connect.Response[ardentsv1.DataInventorySnapshot], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.DataInventorySnapshot, *rpc.Error) {
		return h.dataInventory(), nil
	})
}

func (h *QueryHandler) dataInventory() *ardentsv1.DataInventorySnapshot {
	in := h.content.InventorySnapshot()
	return &ardentsv1.DataInventorySnapshot{
		Objects: int32(in.Objects), Manifests: int32(in.Manifests), Blobs: int32(in.Blobs),
		LocalBlobs: int32(in.LocalBlobs), RemoteBlobs: int32(in.RemoteBlobs), RetainedTemporary: int32(in.RetainedTemporary),
		RelayRetained: int32(in.RelayRetained), Pinned: int32(in.Pinned), Expired: int32(in.Expired), Deleted: int32(in.Deleted),
		Encrypted: int32(in.Encrypted), AvailableForResend: int32(in.AvailableForResend), LocalBytes: in.LocalBytes, RelayBytes: in.RelayBytes,
	}
}

package connectrpc

import (
	dataapi "ardents/internal/data/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toDataInventorySnapshot(in dataapi.DataInventorySnapshot) *ardentsv1.DataInventorySnapshot {
	return &ardentsv1.DataInventorySnapshot{
		Objects:            int32(in.Objects),
		Manifests:          int32(in.Manifests),
		Blobs:              int32(in.Blobs),
		LocalBlobs:         int32(in.LocalBlobs),
		RemoteBlobs:        int32(in.RemoteBlobs),
		RetainedTemporary:  int32(in.RetainedTemporary),
		RelayRetained:      int32(in.RelayRetained),
		Pinned:             int32(in.Pinned),
		Expired:            int32(in.Expired),
		Deleted:            int32(in.Deleted),
		Encrypted:          int32(in.Encrypted),
		AvailableForResend: int32(in.AvailableForResend),
		LocalBytes:         in.LocalBytes,
		RelayBytes:         in.RelayBytes,
	}
}

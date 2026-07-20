package catalog

import (
	dataapi "ardents/internal/data/api"
	model "ardents/internal/data/model"
	observedpkg "ardents/internal/data/observed"
)

type LocalPayloadPresence func(id string) bool

type LocalPayloadInfo func(id string) (bool, int64)

func ObjectPartState(nodeState string, objects map[string]model.Object, manifests map[string]model.Manifest, blobs map[string]model.Blob) (string, string) {
	return observedpkg.ObjectPartState(nodeState, objects, manifests, blobs)
}

func BlobPartState(nodeState string, blobs map[string]model.Blob, hasLocalPayload LocalPayloadPresence) (string, string) {
	return observedpkg.BlobPartState(nodeState, blobs, func(id string) bool {
		return hasLocalPayload(id)
	})
}

func Inventory(objectsCount, manifestsCount int, blobs map[string]model.Blob, localPayload LocalPayloadInfo) model.Inventory {
	return observedpkg.Inventory(objectsCount, manifestsCount, blobs, func(id string) (bool, int64) {
		return localPayload(id)
	})
}

func PartSnapshot(state, reason string) dataapi.PartSnapshot {
	return dataapi.PartSnapshot{State: state, Reason: reason}
}

func InventorySnapshot(in model.Inventory) dataapi.DataInventorySnapshot {
	return dataapi.DataInventorySnapshot{
		Objects:            in.Objects,
		Manifests:          in.Manifests,
		Blobs:              in.Blobs,
		LocalBlobs:         in.LocalBlobs,
		RemoteBlobs:        in.RemoteBlobs,
		RetainedTemporary:  in.RetainedTemporary,
		RelayRetained:      in.RelayRetained,
		Pinned:             in.Pinned,
		Expired:            in.Expired,
		Deleted:            in.Deleted,
		Encrypted:          in.Encrypted,
		AvailableForResend: in.AvailableForResend,
		LocalBytes:         in.LocalBytes,
		RelayBytes:         in.RelayBytes,
	}
}

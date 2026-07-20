package authority

import (
	controlprojection "ardents/internal/control/projection"
	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
)

func objectSnapshot(in appdata.Object) dataapi.ObjectSnapshot {
	return dataapi.ObjectSnapshot{
		ID:        in.ID,
		Type:      in.Type,
		Owner:     in.Owner,
		Body:      controlprojection.CloneMap(in.Body),
		BlobRefs:  refSnapshots(in.BlobRefs),
		CreatedAt: in.CreatedAt,
	}
}

func blobSnapshot(in appdata.Blob) dataapi.BlobSnapshot {
	return dataapi.BlobSnapshot{
		ID:        in.ID,
		CID:       in.CID,
		MediaType: in.MediaType,
		Size:      in.Size,
		Hash:      in.Hash,
		Cipher:    in.Cipher,
		KeyID:     in.KeyID,
		State:     in.State,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		ExpiresAt: in.ExpiresAt,
		CreatedAt: in.CreatedAt,
	}
}

func manifestSnapshot(in appdata.Manifest) dataapi.ManifestSnapshot {
	return dataapi.ManifestSnapshot{
		ID:        in.ID,
		Kind:      in.Kind,
		Owner:     in.Owner,
		Refs:      refSnapshots(in.Refs),
		Access:    in.Access,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		Metadata:  controlprojection.CloneMap(in.Metadata),
		CreatedAt: in.CreatedAt,
	}
}

func inventorySnapshot(in appdata.Inventory) dataapi.DataInventorySnapshot {
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

func refsFromSnapshots(in []dataapi.RefSnapshot) []appdata.Ref {
	if len(in) == 0 {
		return nil
	}
	out := make([]appdata.Ref, 0, len(in))
	for _, item := range in {
		out = append(out, appdata.Ref{Kind: item.Kind, ID: item.ID})
	}
	return out
}

func refSnapshots(in []appdata.Ref) []dataapi.RefSnapshot {
	if len(in) == 0 {
		return nil
	}
	out := make([]dataapi.RefSnapshot, 0, len(in))
	for _, item := range in {
		out = append(out, dataapi.RefSnapshot{Kind: item.Kind, ID: item.ID})
	}
	return out
}

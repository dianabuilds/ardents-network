package data

import (
	dataapi "ardents/internal/data/api"
	catalogpkg "ardents/internal/data/catalog"
	nodeapi "ardents/internal/node/api"
)

func partSnapshot(state, reason string) dataapi.PartSnapshot {
	return catalogpkg.PartSnapshot(state, reason)
}

func inventorySnapshot(in Inventory) dataapi.DataInventorySnapshot {
	return catalogpkg.InventorySnapshot(in)
}

func blobSourceSnapshot(in BlobSourceRecord) dataapi.BlobSourceSnapshot {
	return dataapi.BlobSourceSnapshot{
		BlobID:    in.BlobID,
		NodeID:    in.NodeID,
		ServiceID: in.ServiceID,
		Trust: nodeapi.TrustSnapshot{
			State:   in.Trust.State,
			Outcome: in.Trust.Outcome,
			Valid:   in.Trust.Valid,
			Trusted: in.Trust.Trusted,
			Usable:  in.Trust.Usable,
		},
		Usable:     in.Usable,
		Transport:  in.Transport,
		LastSeenAt: in.LastSeenAt,
		Reason:     in.Reason,
	}
}

func transferSnapshot(in TransferRecord) dataapi.TransferSnapshot {
	return dataapi.TransferSnapshot{
		ID:            in.ID,
		Kind:          in.Kind,
		ResourceID:    in.ResourceID,
		Direction:     in.Direction,
		State:         in.State,
		ProgressBytes: in.ProgressBytes,
		TotalBytes:    in.TotalBytes,
		Peer:          in.Peer,
		StartedAt:     in.StartedAt,
		UpdatedAt:     in.UpdatedAt,
		FinishedAt:    in.FinishedAt,
		Reason:        in.Reason,
	}
}

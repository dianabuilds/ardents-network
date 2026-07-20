package connectrpc

import (
	dataapi "ardents/internal/data/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toBlobSourceSnapshots(items []dataapi.BlobSourceSnapshot) []*ardentsv1.BlobSourceSnapshot {
	out := make([]*ardentsv1.BlobSourceSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &ardentsv1.BlobSourceSnapshot{
			BlobId:     item.BlobID,
			NodeId:     item.NodeID,
			ServiceId:  item.ServiceID,
			Trust:      toTrustSnapshot(item.Trust),
			Usable:     item.Usable,
			Transport:  item.Transport,
			LastSeenAt: ts(item.LastSeenAt),
			Reason:     item.Reason,
		})
	}
	return out
}

func toTransferSnapshot(in dataapi.TransferSnapshot) *ardentsv1.TransferSnapshot {
	return &ardentsv1.TransferSnapshot{
		Id:            in.ID,
		Kind:          in.Kind,
		ResourceId:    in.ResourceID,
		Direction:     in.Direction,
		State:         in.State,
		ProgressBytes: in.ProgressBytes,
		TotalBytes:    in.TotalBytes,
		Peer:          in.Peer,
		StartedAt:     ts(in.StartedAt),
		UpdatedAt:     ts(in.UpdatedAt),
		FinishedAt:    tsp(in.FinishedAt),
		Reason:        in.Reason,
	}
}

func toTransferSnapshots(items []dataapi.TransferSnapshot) []*ardentsv1.TransferSnapshot {
	out := make([]*ardentsv1.TransferSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, toTransferSnapshot(item))
	}
	return out
}

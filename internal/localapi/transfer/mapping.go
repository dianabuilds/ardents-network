package transfer

import (
	domain "ardents/internal/content"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
	transferdomain "ardents/internal/transfer"
)

func blobSnapshot(in domain.Blob) *ardentsv1.BlobSnapshot {
	return &ardentsv1.BlobSnapshot{Id: in.ID, MediaType: in.MediaType, Size: in.Size, Hash: in.Hash,
		CreatedAt: rpc.Timestamp(in.CreatedAt), Cid: in.CID, Cipher: in.Cipher, KeyId: in.KeyID,
		State: in.State, Retention: in.Retention, Encrypted: in.Encrypted, ExpiresAt: rpc.Timestamp(in.ExpiresAt)}
}

func sourceSnapshots(items []domain.BlobSourceRecord) []*ardentsv1.BlobSourceSnapshot {
	out := make([]*ardentsv1.BlobSourceSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &ardentsv1.BlobSourceSnapshot{BlobId: item.BlobID, NodeId: item.NodeID, ServiceId: item.ServiceID,
			Trust:  &ardentsv1.TrustSnapshot{State: item.Trust.State, Outcome: item.Trust.Outcome, Valid: item.Trust.Valid, Trusted: item.Trust.Trusted, Usable: item.Trust.Usable},
			Usable: item.Usable, Transport: item.Transport, LastSeenAt: rpc.Timestamp(item.LastSeenAt), Reason: item.Reason})
	}
	return out
}

func transferSnapshot(in transferdomain.Record) *ardentsv1.TransferSnapshot {
	return &ardentsv1.TransferSnapshot{Id: in.ID, Kind: in.Kind, ResourceId: in.ResourceID, Direction: in.Direction,
		State: in.State, ProgressBytes: in.ProgressBytes, TotalBytes: in.TotalBytes, Peer: in.Peer,
		StartedAt: rpc.Timestamp(in.StartedAt), UpdatedAt: rpc.Timestamp(in.UpdatedAt), FinishedAt: rpc.TimestampPointer(in.FinishedAt), Reason: in.Reason}
}

func transferSnapshots(items []transferdomain.Record) []*ardentsv1.TransferSnapshot {
	out := make([]*ardentsv1.TransferSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, transferSnapshot(item))
	}
	return out
}

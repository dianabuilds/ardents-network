package content

import (
	appdata "ardents/internal/content"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func toBlobSnapshot(in appdata.Blob) *ardentsv1.BlobSnapshot {
	return &ardentsv1.BlobSnapshot{
		Id:        in.ID,
		MediaType: in.MediaType,
		Size:      in.Size,
		Hash:      in.Hash,
		CreatedAt: rpc.Timestamp(in.CreatedAt),
		Cid:       in.CID,
		Cipher:    in.Cipher,
		KeyId:     in.KeyID,
		State:     in.State,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		ExpiresAt: rpc.Timestamp(in.ExpiresAt),
	}
}

func fromBlobSnapshot(in *ardentsv1.BlobSnapshot) appdata.PublishBlobCommand {
	if in == nil {
		return appdata.PublishBlobCommand{}
	}
	return appdata.PublishBlobCommand{Blob: appdata.Blob{
		ID:        in.GetId(),
		CID:       in.GetCid(),
		MediaType: in.GetMediaType(),
		Size:      in.GetSize(),
		Hash:      in.GetHash(),
		Cipher:    in.GetCipher(),
		KeyID:     in.GetKeyId(),
		State:     in.GetState(),
		Retention: in.GetRetention(),
		Encrypted: in.GetEncrypted(),
		ExpiresAt: rpc.Time(in.GetExpiresAt()),
		CreatedAt: rpc.Time(in.GetCreatedAt()),
	}, Payload: append([]byte(nil), in.GetPayload()...)}
}

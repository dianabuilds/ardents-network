package connectrpc

import (
	dataapi "ardents/internal/data/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toBlobSnapshot(in dataapi.BlobSnapshot) *ardentsv1.BlobSnapshot {
	return &ardentsv1.BlobSnapshot{
		Id:        in.ID,
		MediaType: in.MediaType,
		Size:      in.Size,
		Payload:   append([]byte(nil), in.Payload...),
		Hash:      in.Hash,
		CreatedAt: ts(in.CreatedAt),
		Cid:       in.CID,
		Cipher:    in.Cipher,
		KeyId:     in.KeyID,
		State:     in.State,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		ExpiresAt: ts(in.ExpiresAt),
	}
}

func fromBlobSnapshot(in *ardentsv1.BlobSnapshot) dataapi.BlobSnapshot {
	if in == nil {
		return dataapi.BlobSnapshot{}
	}
	return dataapi.BlobSnapshot{
		ID:        in.GetId(),
		CID:       in.GetCid(),
		MediaType: in.GetMediaType(),
		Size:      in.GetSize(),
		Payload:   append([]byte(nil), in.GetPayload()...),
		Hash:      in.GetHash(),
		Cipher:    in.GetCipher(),
		KeyID:     in.GetKeyId(),
		State:     in.GetState(),
		Retention: in.GetRetention(),
		Encrypted: in.GetEncrypted(),
		ExpiresAt: fromTS(in.GetExpiresAt()),
		CreatedAt: fromTS(in.GetCreatedAt()),
	}
}

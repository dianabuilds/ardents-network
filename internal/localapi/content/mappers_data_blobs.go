package content

import (
	appdata "ardents/internal/content"
	model "ardents/internal/content/catalog"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func toBlobSnapshot(in appdata.Blob) *ardentsv1.BlobSnapshot {
	return &ardentsv1.BlobSnapshot{
		Reference: in.Reference.String(),
		MediaType: in.MediaType,
		Size:      in.Size,
		Hash:      in.Hash,
		CreatedAt: rpc.Timestamp(in.CreatedAt),
		Cipher:    in.Cipher,
		KeyId:     in.KeyID,
		State:     in.State,
		Retention: in.Retention,
		Encrypted: in.Encrypted,
		ExpiresAt: rpc.Timestamp(in.ExpiresAt),
	}
}

func fromBlobSnapshot(in *ardentsv1.BlobSnapshot) (appdata.PublishBlobCommand, error) {
	if in == nil {
		return appdata.PublishBlobCommand{}, nil
	}
	var reference model.ContentReference
	if in.GetReference() != "" {
		parsed, err := model.ParseContentReference(in.GetReference())
		if err != nil {
			return appdata.PublishBlobCommand{}, err
		}
		reference = parsed
	}
	return appdata.PublishBlobCommand{Blob: appdata.Blob{
		Reference: reference,
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
	}, Payload: append([]byte(nil), in.GetPayload()...)}, nil
}

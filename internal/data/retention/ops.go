package retention

import (
	"fmt"
	"time"

	dataapi "ardents/internal/data/api"
	model "ardents/internal/data/model"
	datapayload "ardents/internal/data/payload"
	statepkg "ardents/internal/data/state"
)

type Authorizer func(blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time) error

type LocalPayloadInfo func(id string) (bool, int64)

func ResolveRelayExpiry(now, expiresAt time.Time, defaultTTL time.Duration) (time.Time, error) {
	if !expiresAt.IsZero() {
		return expiresAt.UTC(), nil
	}
	if defaultTTL <= 0 {
		return time.Time{}, fmt.Errorf("relay retention expiry is required")
	}
	return now.Add(defaultTTL), nil
}

func PrepareRelayBlob(blob model.Blob, payload []byte, expiresAt time.Time, authorize Authorizer) (model.Blob, error) {
	blob = NormalizeBlob(blob)
	if !blob.Encrypted {
		return model.Blob{}, fmt.Errorf("relay retention requires encrypted blob")
	}
	if authorize != nil {
		if err := authorize(BlobPolicySnapshot(blob), true, expiresAt.UTC()); err != nil {
			return model.Blob{}, err
		}
	}
	hash, blobCID, err := datapayload.DeriveIdentity(payload)
	if err != nil {
		return model.Blob{}, err
	}
	if err := datapayload.ApplyDerivedIdentity(&blob, hash, blobCID); err != nil {
		return model.Blob{}, err
	}
	blob.Size = int64(len(payload))
	blob.State = "retained-temporary"
	blob.Retention = "relay-temporary"
	blob.ExpiresAt = expiresAt.UTC()
	return blob, nil
}

func RetainBlob(
	blobs *statepkg.BlobStore,
	id string,
	expiresAt time.Time,
	now time.Time,
	defaultTTL time.Duration,
	hasLocalPayload func(string) bool,
	authorize Authorizer,
) (model.Blob, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("blob not found")
	}
	if !hasLocalPayload(id) {
		return model.Blob{}, fmt.Errorf("blob payload is not locally available")
	}
	if expiresAt.IsZero() {
		if defaultTTL <= 0 {
			return model.Blob{}, fmt.Errorf("retention expiry is required")
		}
		expiresAt = now.Add(defaultTTL)
	}
	if authorize != nil {
		if err := authorize(BlobPolicySnapshot(blob), false, expiresAt.UTC()); err != nil {
			return model.Blob{}, err
		}
	}
	blob.State = "retained-temporary"
	blob.Retention = "temporary"
	blob.ExpiresAt = expiresAt.UTC()
	blobs.Put(blob)
	return blob, nil
}

func PinBlob(blobs *statepkg.BlobStore, id string, hasLocalPayload func(string) bool) (model.Blob, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("blob not found")
	}
	if !hasLocalPayload(id) {
		return model.Blob{}, fmt.Errorf("blob payload is not locally available")
	}
	blob.State = "pinned"
	blob.Retention = "pinned"
	blob.ExpiresAt = time.Time{}
	blobs.Put(blob)
	return blob, nil
}

func DropBlob(blobs *statepkg.BlobStore, id string, removePayload func(string) error) (model.Blob, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("blob not found")
	}
	if err := removePayload(id); err != nil {
		return model.Blob{}, err
	}
	blob.State = "deleted"
	blob.ExpiresAt = time.Time{}
	blobs.Put(blob)
	return blob, nil
}

func PruneExpired(blobs *statepkg.BlobStore, now time.Time, removePayload func(string) error) ([]string, bool, error) {
	var (
		changed   bool
		prunedIDs []string
	)
	for id, blob := range blobs.Items {
		if blob.State != "retained-temporary" || blob.ExpiresAt.IsZero() || blob.ExpiresAt.After(now) {
			continue
		}
		if err := removePayload(id); err != nil {
			return nil, false, err
		}
		blob.State = "expired"
		blobs.Put(blob)
		prunedIDs = append(prunedIDs, id)
		changed = true
	}
	return prunedIDs, changed, nil
}

func RelayBytes(blobs *statepkg.BlobStore, excludeID string, localPayloadInfo LocalPayloadInfo) int64 {
	var total int64
	for id, blob := range blobs.Items {
		if id == excludeID || blob.Retention != "relay-temporary" || blob.State == "deleted" || blob.State == "expired" {
			continue
		}
		total += relayBlobBytes(id, blob, localPayloadInfo)
	}
	return total
}

func NormalizeBlob(blob model.Blob) model.Blob {
	if blob.Retention == "" {
		blob.Retention = "owner"
	}
	return blob
}

func BlobPolicySnapshot(blob model.Blob) dataapi.BlobSnapshot {
	return dataapi.BlobSnapshot{
		ID:        blob.ID,
		CID:       blob.CID,
		MediaType: blob.MediaType,
		Size:      blob.Size,
		Hash:      blob.Hash,
		Cipher:    blob.Cipher,
		KeyID:     blob.KeyID,
		State:     blob.State,
		Retention: blob.Retention,
		Encrypted: blob.Encrypted,
		ExpiresAt: blob.ExpiresAt,
		CreatedAt: blob.CreatedAt,
	}
}

func relayBlobBytes(id string, blob model.Blob, localPayloadInfo LocalPayloadInfo) int64 {
	if blob.Size != 0 {
		return blob.Size
	}
	present, size := localPayloadInfo(id)
	if !present {
		return 0
	}
	return size
}

package blob

import (
	"fmt"
	"time"

	model "ardents/internal/data/model"
	datapayload "ardents/internal/data/payload"
	statepkg "ardents/internal/data/state"
)

type Source interface {
	GetBlob(string) (model.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
}

func Publish(blobs *statepkg.BlobStore, blob model.Blob, nextID func(string) string, writePayload func(string, []byte) error) (model.Blob, error) {
	return Store(blobs, blob, nil, nextID, writePayload)
}

func Store(
	blobs *statepkg.BlobStore,
	blob model.Blob,
	payload []byte,
	nextID func(string) string,
	writePayload func(string, []byte) error,
) (model.Blob, error) {
	blob = Normalize(blob)
	if len(payload) != 0 {
		if err := populateStoredBlob(&blob, payload, writePayload); err != nil {
			return model.Blob{}, err
		}
	} else if err := prepareMetadataOnlyBlob(&blob, nextID); err != nil {
		return model.Blob{}, err
	}
	if blob.CreatedAt.IsZero() {
		blob.CreatedAt = time.Now().UTC()
	}
	blobs.Put(blob)
	return blob, nil
}

func AnnounceRemote(blobs *statepkg.BlobStore, blob model.Blob) (model.Blob, error) {
	blob = Normalize(blob)
	if err := datapayload.ValidateMetadataIdentity(blob); err != nil {
		return model.Blob{}, err
	}
	if blob.ID == "" {
		if blob.CID != "" {
			blob.ID = blob.CID
		} else {
			return model.Blob{}, fmt.Errorf("remote blob id is required")
		}
	}
	if blob.State == "" || blob.State == "announced" || datapayload.StateRequiresLocalPayload(blob.State) {
		blob.State = "available-remote"
	}
	blobs.Put(blob)
	return blob, nil
}

func Fetch(id string, source Source, store func(model.Blob, []byte) (model.Blob, error)) (model.Blob, error) {
	if source == nil {
		return model.Blob{}, fmt.Errorf("blob source is required")
	}
	meta, ok := source.GetBlob(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("remote blob not found")
	}
	payload, err := source.GetBlobPayload(id)
	if err != nil {
		return model.Blob{}, err
	}
	meta.State = "available-local"
	if meta.Retention == "" {
		meta.Retention = "fetched"
	}
	return store(meta, payload)
}

func Get(blobs *statepkg.BlobStore, id string) (model.Blob, bool) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, false
	}
	return blob, true
}

func Payload(blobs *statepkg.BlobStore, id string, readPayload func(string) ([]byte, error)) ([]byte, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return nil, fmt.Errorf("blob not found")
	}
	if !datapayload.StateRequiresLocalPayload(blob.State) {
		return nil, fmt.Errorf("blob payload is not locally available")
	}
	return readPayload(id)
}

func List(blobs *statepkg.BlobStore, sortedKeys func(map[string]model.Blob) []string) []model.Blob {
	ids := sortedKeys(blobs.Items)
	out := make([]model.Blob, 0, len(ids))
	for _, id := range ids {
		out = append(out, blobs.Items[id])
	}
	return out
}

func Normalize(blob model.Blob) model.Blob {
	if blob.Retention == "" {
		blob.Retention = "owner"
	}
	return blob
}

func populateStoredBlob(blob *model.Blob, payload []byte, writePayload func(string, []byte) error) error {
	hash, blobCID, err := datapayload.DeriveIdentity(payload)
	if err != nil {
		return err
	}
	if err := datapayload.ApplyDerivedIdentity(blob, hash, blobCID); err != nil {
		return err
	}
	blob.Size = int64(len(payload))
	blob.State = "available-local"
	return writePayload(blob.ID, payload)
}

func prepareMetadataOnlyBlob(blob *model.Blob, nextID func(string) string) error {
	if err := datapayload.ValidateMetadataIdentity(*blob); err != nil {
		return err
	}
	if blob.ID == "" {
		if blob.CID != "" {
			blob.ID = blob.CID
		} else {
			blob.ID = nextID("blob")
		}
	}
	if blob.State == "" {
		blob.State = "announced"
		return nil
	}
	if datapayload.StateRequiresLocalPayload(blob.State) {
		return fmt.Errorf("blob state %q requires a local payload", blob.State)
	}
	return nil
}

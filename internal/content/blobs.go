// Package content owns local content lifecycle, inventory, retention, and orchestration.
// It does not own peer exchange or replica placement.
package content

import (
	model "ardents/internal/content/catalog"
	datapayload "ardents/internal/content/payload"
	db "ardents/internal/storage"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Source interface {
	GetBlob(string) (model.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
}

func Store(
	blobs *model.BlobStore,
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

func AnnounceRemote(blobs *model.BlobStore, blob model.Blob) (model.Blob, error) {
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

func Get(blobs *model.BlobStore, id string) (model.Blob, bool) {
	blob, ok := blobs.Get(id)
	if !ok {
		return model.Blob{}, false
	}
	return blob, true
}

func Payload(blobs *model.BlobStore, id string, readPayload func(string) ([]byte, error)) ([]byte, error) {
	blob, ok := blobs.Get(id)
	if !ok {
		return nil, fmt.Errorf("blob not found")
	}
	if !datapayload.StateRequiresLocalPayload(blob.State) {
		return nil, fmt.Errorf("blob payload is not locally available")
	}
	return readPayload(id)
}

func List(blobs *model.BlobStore, sortedKeys func(map[string]model.Blob) []string) []model.Blob {
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

func (s *Service) PublishBlob(blob Blob) (Blob, error) {
	return s.StoreBlob(blob, nil)
}

func (s *Service) StoreBlob(blob Blob, payload []byte) (Blob, error) {
	if blob.Retention == "staging" {
		return Blob{}, fmt.Errorf("staging retention is reserved for chunk assembly")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := s.blobs.Snapshot(), s.state
	blob, err := Store(&s.blobs, blob, payload, s.nextID, s.writePayloadLocked)
	if err != nil {
		return Blob{}, err
	}
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		if len(payload) == 0 {
			s.restoreBlobSnapshotLocked(previous, previousState)
			return Blob{}, err
		}
		return Blob{}, errors.Join(err, s.rollbackUncommittedPayloadLocked(previous, previousState, blob.ID))
	}
	return blob, nil
}

func (s *Service) AnnounceRemoteBlob(blob Blob) (Blob, error) {
	if blob.Retention == "staging" || blob.State == "staging" {
		return Blob{}, fmt.Errorf("staging blob cannot be announced")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := s.blobs.Snapshot(), s.state
	blob, err := AnnounceRemote(&s.blobs, blob)
	if err != nil {
		return Blob{}, err
	}
	s.state = "ready"
	if err := s.saveLocked(); err != nil {
		s.restoreBlobSnapshotLocked(previous, previousState)
		return Blob{}, err
	}
	return blob, nil
}

func (s *Service) FetchBlob(id string, source BlobSource) (Blob, error) {
	return Fetch(id, source, s.StoreBlob)
}

func (s *Service) GetBlob(id string) (Blob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Get(&s.blobs, id)
}

func (s *Service) GetBlobPayload(id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Payload(&s.blobs, id, func(id string) ([]byte, error) {
		return s.readPayloadLocked(id)
	})
}

func (s *Service) ListBlobs() []Blob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return List(&s.blobs, sortedKeys[Blob])
}

func (s *Service) nextID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

func (s *Service) writePayloadLocked(id string, payload []byte) error {
	if s.cfg.MaxLocalStorageBytes > 0 {
		used := s.localPayloadBytesLocked(id)
		if used+int64(len(payload)) > s.cfg.MaxLocalStorageBytes {
			return fmt.Errorf("local storage capacity exceeded")
		}
	}
	return db.AtomicWritePrivateFile(s.payloadPath(id), payload)
}

func (s *Service) localPayloadBytesLocked(replacingID string) int64 {
	var total int64
	for id, blob := range s.blobs.Snapshot() {
		if id != replacingID && (datapayload.StateRequiresLocalPayload(blob.State) || blob.State == "staging") {
			total += blob.Size
		}
	}
	return total
}

func (s *Service) readPayloadLocked(id string) ([]byte, error) {
	raw, found, err := db.ReadProtectedFile(s.payloadPath(id))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, os.ErrNotExist
	}
	return raw, nil
}

func (s *Service) payloadPath(id string) string {
	safeID := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(id)
	return filepath.Join(s.dir, "blobs", safeID+".blob")
}

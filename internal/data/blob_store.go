package data

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	blobpkg "ardents/internal/data/blob"
	datapayload "ardents/internal/data/payload"
	db "ardents/internal/persistence"
)

func (s *Service) PublishBlob(blob Blob) (Blob, error) {
	return s.StoreBlob(blob, nil)
}

func (s *Service) StoreBlob(blob Blob, payload []byte) (Blob, error) {
	if blob.Retention == "staging" {
		return Blob{}, fmt.Errorf("staging retention is reserved for chunk assembly")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	previous, previousState := maps.Clone(s.blobs.Items), s.state
	blob, err := blobpkg.Store(&s.blobs, blob, payload, s.nextID, s.writePayloadLocked)
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

	previous, previousState := maps.Clone(s.blobs.Items), s.state
	blob, err := blobpkg.AnnounceRemote(&s.blobs, blob)
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
	return blobpkg.Fetch(id, source, s.StoreBlob)
}

func (s *Service) GetBlob(id string) (Blob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return blobpkg.Get(&s.blobs, id)
}

func (s *Service) GetBlobPayload(id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return blobpkg.Payload(&s.blobs, id, func(id string) ([]byte, error) {
		return s.readPayloadLocked(id)
	})
}

func (s *Service) ListBlobs() []Blob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return blobpkg.List(&s.blobs, sortedKeys[Blob])
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
	for id, blob := range s.blobs.Items {
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

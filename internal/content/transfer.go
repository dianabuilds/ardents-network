package content

import (
	"ardents/internal/content/catalog"
	"time"
)

func (s *Service) ObserveBlobSource(blobID string, source BlobSourceRecord) (BlobSourceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, err := catalog.ObserveSource(&s.blobs, &s.sources, blobID, source)
	if err != nil {
		return BlobSourceRecord{}, err
	}
	s.state = "ready"
	return source, s.saveLocked()
}

func (s *Service) ListBlobSources(blobID string) []BlobSourceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return catalog.ListSources(&s.blobs, &s.sources, blobID, s.localNodeID, time.Now().UTC())
}

func (s *Service) relayBytesWithoutLocked(excludeID string) int64 {
	return RelayBytes(&s.blobs, excludeID, s.localPayloadInfoLocked)
}

func (s *Service) ReadTransferManifest(id string) (catalog.Manifest, bool) {
	manifest, ok := s.GetManifest(id)
	if !ok {
		return catalog.Manifest{}, false
	}
	return manifestModel(manifest), true
}

func (s *Service) WriteTransferManifest(manifest catalog.Manifest) (catalog.Manifest, error) {
	stored, err := s.PublishManifest(manifestSnapshot(manifest))
	if err != nil {
		return catalog.Manifest{}, err
	}
	return manifestModel(stored), nil
}

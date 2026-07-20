package data

import (
	"time"

	dataapi "ardents/internal/data/api"
	catalogpkg "ardents/internal/data/catalog"
)

func (s *Service) ObserveBlobSource(blobID string, source BlobSourceRecord) (BlobSourceRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	source, err := catalogpkg.ObserveSource(&s.blobs, &s.sources, blobID, source)
	if err != nil {
		return BlobSourceRecord{}, err
	}
	s.state = "ready"
	return source, s.saveLocked()
}

func (s *Service) ListBlobSources(blobID string) []BlobSourceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return catalogpkg.ListSources(&s.blobs, &s.sources, blobID, s.localNodeID, time.Now().UTC())
}

func (s *Service) ListBlobSourceSnapshots(blobID string) []dataapi.BlobSourceSnapshot {
	items := s.ListBlobSources(blobID)
	out := make([]dataapi.BlobSourceSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, blobSourceSnapshot(item))
	}
	return out
}

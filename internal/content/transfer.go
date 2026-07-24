package content

import (
	"ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
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

func (s *Service) ReadTransferManifest(owner principal.ID, id string) (catalog.Manifest, bool) {
	manifest, ok := s.GetManifest(owner, id)
	if !ok {
		return catalog.Manifest{}, false
	}
	return manifestModel(manifest), true
}

// ResolveLegacyTransferManifestOwner exists only for the replication v2 to v3
// state migration. Runtime content access must always provide the owner.
func (s *Service) ResolveLegacyTransferManifestOwner(id string) (principal.ID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var owner principal.ID
	found := false
	for _, manifest := range s.manifests.Items {
		if manifest.ID != id {
			continue
		}
		if found && !manifest.Owner.Equal(owner) {
			return principal.ID{}, false
		}
		owner, found = manifest.Owner, true
	}
	return owner, found
}

func (s *Service) WriteTransferManifest(manifest catalog.Manifest) (catalog.Manifest, error) {
	stored, err := s.PublishManifest(manifestSnapshot(manifest))
	if err != nil {
		return catalog.Manifest{}, err
	}
	return manifestModel(stored), nil
}

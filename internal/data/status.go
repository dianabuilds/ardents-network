package data

import (
	"os"

	dataapi "ardents/internal/data/api"
	catalogpkg "ardents/internal/data/catalog"
)

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) ObjectState() string {
	state, _ := s.ObjectPartState()
	return state
}

func (s *Service) BlobState() string {
	state, _ := s.BlobPartState()
	return state
}

func (s *Service) ManifestState() string {
	state, _ := s.ObjectPartState()
	return state
}

func (s *Service) ObjectPartState() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return catalogpkg.ObjectPartState(s.state, s.objects.Items, s.manifests.Items, s.blobs.Items)
}

func (s *Service) BlobPartState() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return catalogpkg.BlobPartState(s.state, s.blobs.Items, s.hasLocalPayloadLocked)
}

func (s *Service) Inventory() Inventory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return catalogpkg.Inventory(s.objects.Count(), s.manifests.Count(), s.blobs.Items, s.localPayloadInfoLocked)
}

func (s *Service) ObjectPart() dataapi.PartSnapshot {
	state, reason := s.ObjectPartState()
	return partSnapshot(state, reason)
}

func (s *Service) BlobPart() dataapi.PartSnapshot {
	state, reason := s.BlobPartState()
	return partSnapshot(state, reason)
}

func (s *Service) DataInventory() dataapi.DataInventorySnapshot {
	return inventorySnapshot(s.Inventory())
}

func (s *Service) localPayloadInfoLocked(id string) (bool, int64) {
	info, err := os.Stat(s.payloadPath(id))
	if err != nil {
		return false, 0
	}
	return true, info.Size()
}

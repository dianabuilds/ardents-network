package data

import (
	manifestpkg "ardents/internal/data/manifest"
)

func (s *Service) PublishManifest(manifest Manifest) (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	manifest, err := manifestpkg.Publish(&s.manifests, &s.blobs, manifest, s.nextID)
	if err != nil {
		return Manifest{}, err
	}
	s.state = "ready"
	return manifest, s.saveLocked()
}

func (s *Service) GetManifest(id string) (Manifest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return manifestpkg.Get(&s.manifests, id)
}

func (s *Service) ListManifests() []Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return manifestpkg.List(&s.manifests, sortedKeys[Manifest])
}

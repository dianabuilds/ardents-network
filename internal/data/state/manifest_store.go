package state

import model "ardents/internal/data/model"

type ManifestStore struct {
	Items map[string]model.Manifest
}

func NewManifestStore() ManifestStore {
	return ManifestStore{Items: map[string]model.Manifest{}}
}

func (s *ManifestStore) Load(items map[string]model.Manifest) {
	if items == nil {
		s.Items = map[string]model.Manifest{}
		return
	}
	s.Items = items
}

func (s *ManifestStore) Snapshot() map[string]model.Manifest {
	return s.Items
}

func (s *ManifestStore) Get(id string) (model.Manifest, bool) {
	item, ok := s.Items[id]
	return item, ok
}

func (s *ManifestStore) Put(item model.Manifest) {
	s.Items[item.ID] = item
}

func (s *ManifestStore) Delete(id string) {
	delete(s.Items, id)
}

func (s *ManifestStore) Count() int {
	return len(s.Items)
}

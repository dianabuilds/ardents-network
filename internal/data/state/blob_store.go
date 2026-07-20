package state

import model "ardents/internal/data/model"

type BlobStore struct {
	Items map[string]model.Blob
}

func NewBlobStore() BlobStore {
	return BlobStore{Items: map[string]model.Blob{}}
}

func (s *BlobStore) Load(items map[string]model.Blob) {
	if items == nil {
		s.Items = map[string]model.Blob{}
		return
	}
	s.Items = items
}

func (s *BlobStore) Snapshot() map[string]model.Blob {
	return s.Items
}

func (s *BlobStore) Get(id string) (model.Blob, bool) {
	item, ok := s.Items[id]
	return item, ok
}

func (s *BlobStore) Put(item model.Blob) {
	s.Items[item.ID] = item
}

func (s *BlobStore) Delete(id string) {
	delete(s.Items, id)
}

func (s *BlobStore) Count() int {
	return len(s.Items)
}

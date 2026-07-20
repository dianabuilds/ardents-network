package state

import model "ardents/internal/data/model"

type ObjectStore struct {
	Items map[string]model.Object
}

func NewObjectStore() ObjectStore {
	return ObjectStore{Items: map[string]model.Object{}}
}

func (s *ObjectStore) Load(items map[string]model.Object) {
	if items == nil {
		s.Items = map[string]model.Object{}
		return
	}
	s.Items = items
}

func (s *ObjectStore) Snapshot() map[string]model.Object {
	return s.Items
}

func (s *ObjectStore) Get(id string) (model.Object, bool) {
	item, ok := s.Items[id]
	return item, ok
}

func (s *ObjectStore) Put(item model.Object) {
	s.Items[item.ID] = item
}

func (s *ObjectStore) Count() int {
	return len(s.Items)
}

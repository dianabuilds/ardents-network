package data

import (
	objectpkg "ardents/internal/data/object"
)

func (s *Service) PublishObject(object Object) (Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	object, err := objectpkg.Publish(&s.objects, &s.blobs, object, s.nextID)
	if err != nil {
		return Object{}, err
	}
	s.state = "ready"
	return object, s.saveLocked()
}

func (s *Service) GetObject(id string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return objectpkg.Get(&s.objects, id)
}

func (s *Service) ListObjects() []Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	return objectpkg.List(&s.objects, sortedKeys[Object])
}

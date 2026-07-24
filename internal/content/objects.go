package content

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
	"fmt"
	"time"
)

func (s *Service) PublishObject(object Object) (Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := publishObjectModel(&s.objects, &s.blobs, objectModel(object), s.nextID, s.now())
	if err != nil {
		return Object{}, err
	}
	s.state = "ready"
	return objectSnapshot(stored), s.saveLocked()
}

func (s *Service) GetObject(owner principal.ID, id string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := getObjectModel(&s.objects, owner, id)
	return objectSnapshot(stored), ok
}

func (s *Service) ListObjects(owner principal.ID) []Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Object, 0)
	for _, item := range listObjectModels(&s.objects, sortedKeys[model.Object]) {
		if item.Owner.Equal(owner) {
			items = append(items, objectSnapshot(item))
		}
	}
	return items
}

func publishObjectModel(objects *model.ObjectStore, blobs *model.BlobStore, object model.Object, nextID func(string) string, now time.Time) (model.Object, error) {
	object = normalizeObjectModel(object)
	if object.Owner.String() == "" {
		return model.Object{}, fmt.Errorf("object owner is required")
	}
	for _, ref := range object.BlobRefs {
		if ref.ID == "" || ref.Kind == "" {
			return model.Object{}, fmt.Errorf("object ref is incomplete")
		}
		if ref.Kind == "blob" {
			if _, ok := blobs.Get(ref.ID); !ok {
				return model.Object{}, fmt.Errorf("blob ref %q not found", ref.ID)
			}
		}
	}
	if object.ID == "" {
		object.ID = nextID("obj")
	}
	if object.CreatedAt.IsZero() {
		object.CreatedAt = now.UTC()
	}
	objects.Put(object)
	return cloneObjectModel(object), nil
}

func getObjectModel(objects *model.ObjectStore, owner principal.ID, id string) (model.Object, bool) {
	object, ok := objects.Get(owner, id)
	return cloneObjectModel(object), ok
}

func listObjectModels(objects *model.ObjectStore, keys func(map[string]model.Object) []string) []model.Object {
	out := make([]model.Object, 0, len(objects.Items))
	for _, id := range keys(objects.Items) {
		out = append(out, cloneObjectModel(objects.Items[id]))
	}
	return out
}

func normalizeObjectModel(object model.Object) model.Object {
	if object.Body == nil {
		object.Body = map[string]any{}
	}
	if object.BlobRefs == nil {
		object.BlobRefs = []model.Ref{}
	}
	return object
}

func cloneObjectModel(object model.Object) model.Object {
	object.Body = cloneMap(object.Body)
	object.BlobRefs = append([]model.Ref(nil), object.BlobRefs...)
	return object
}

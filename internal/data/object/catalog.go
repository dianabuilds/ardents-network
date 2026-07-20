package object

import (
	"fmt"
	"time"

	model "ardents/internal/data/model"
	statepkg "ardents/internal/data/state"
)

func Publish(objects *statepkg.ObjectStore, blobs *statepkg.BlobStore, object model.Object, nextID func(string) string) (model.Object, error) {
	object = Normalize(object)
	if err := ValidateBlobRefs(blobs, object.BlobRefs); err != nil {
		return model.Object{}, err
	}
	if object.ID == "" {
		object.ID = nextID("obj")
	}
	if object.CreatedAt.IsZero() {
		object.CreatedAt = time.Now().UTC()
	}
	objects.Put(object)
	return Clone(object), nil
}

func Get(objects *statepkg.ObjectStore, id string) (model.Object, bool) {
	object, ok := objects.Get(id)
	if !ok {
		return model.Object{}, false
	}
	return Clone(object), true
}

func List(objects *statepkg.ObjectStore, sortedKeys func(map[string]model.Object) []string) []model.Object {
	ids := sortedKeys(objects.Items)
	out := make([]model.Object, 0, len(ids))
	for _, id := range ids {
		out = append(out, Clone(objects.Items[id]))
	}
	return out
}

func ValidateBlobRefs(blobs *statepkg.BlobStore, refs []model.Ref) error {
	for _, ref := range refs {
		if ref.ID == "" || ref.Kind == "" {
			return fmt.Errorf("object ref is incomplete")
		}
		if ref.Kind == "blob" {
			if _, ok := blobs.Get(ref.ID); !ok {
				return fmt.Errorf("blob ref %q not found", ref.ID)
			}
		}
	}
	return nil
}

func Normalize(object model.Object) model.Object {
	if object.Body == nil {
		object.Body = map[string]any{}
	}
	if object.BlobRefs == nil {
		object.BlobRefs = []model.Ref{}
	}
	return object
}

func Clone(object model.Object) model.Object {
	object.Body = cloneMap(object.Body)
	object.BlobRefs = append([]model.Ref(nil), object.BlobRefs...)
	return object
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

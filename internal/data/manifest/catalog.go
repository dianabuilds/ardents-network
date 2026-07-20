package manifest

import (
	"fmt"
	"time"

	"ardents/internal/data/chunking"
	model "ardents/internal/data/model"
	statepkg "ardents/internal/data/state"
)

func Publish(manifests *statepkg.ManifestStore, blobs *statepkg.BlobStore, manifest model.Manifest, nextID func(string) string) (model.Manifest, error) {
	manifest = Normalize(manifest)
	if manifest.Kind == "chunk-leaf" || manifest.Kind == "chunk-root" {
		if err := chunking.ValidateManifest(manifest); err != nil {
			return model.Manifest{}, err
		}
	}
	if manifest.ID == "" {
		manifest.ID = nextID("manifest")
	}
	for _, ref := range manifest.Refs {
		if ref.ID == "" || ref.Kind == "" {
			return model.Manifest{}, fmt.Errorf("manifest ref is incomplete")
		}
		if ref.Kind == "blob" {
			if _, ok := blobs.Get(ref.ID); !ok {
				return model.Manifest{}, fmt.Errorf("blob ref %q not found", ref.ID)
			}
		}
		if ref.Kind == "manifest" {
			if _, ok := manifests.Get(ref.ID); !ok {
				return model.Manifest{}, fmt.Errorf("manifest ref %q not found", ref.ID)
			}
		}
	}
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = time.Now().UTC()
	}
	manifests.Put(manifest)
	return Clone(manifest), nil
}

func Get(manifests *statepkg.ManifestStore, id string) (model.Manifest, bool) {
	manifest, ok := manifests.Get(id)
	if !ok {
		return model.Manifest{}, false
	}
	return Clone(manifest), true
}

func List(manifests *statepkg.ManifestStore, sortedKeys func(map[string]model.Manifest) []string) []model.Manifest {
	ids := sortedKeys(manifests.Items)
	out := make([]model.Manifest, 0, len(ids))
	for _, id := range ids {
		out = append(out, Clone(manifests.Items[id]))
	}
	return out
}

func Normalize(manifest model.Manifest) model.Manifest {
	if manifest.Kind == "" {
		manifest.Kind = "blob-set"
	}
	if manifest.Access == "" {
		manifest.Access = "participants"
	}
	if manifest.Retention == "" {
		manifest.Retention = "owner"
	}
	if manifest.Metadata == nil {
		manifest.Metadata = map[string]any{}
	}
	if manifest.Refs == nil {
		manifest.Refs = []model.Ref{}
	}
	return manifest
}

func Clone(manifest model.Manifest) model.Manifest {
	manifest.Metadata = cloneMap(manifest.Metadata)
	manifest.Refs = append([]model.Ref(nil), manifest.Refs...)
	return manifest
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

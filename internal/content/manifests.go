package content

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
	"fmt"
	"time"
)

func (s *Service) PublishManifest(manifest Manifest) (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := publishManifestModel(&s.manifests, &s.blobs, manifestModel(manifest), s.nextID, s.now())
	if err != nil {
		return Manifest{}, err
	}
	s.state = "ready"
	return manifestSnapshot(stored), s.saveLocked()
}

func (s *Service) GetManifest(owner principal.ID, id string) (Manifest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := getManifestModel(&s.manifests, owner, id)
	return manifestSnapshot(stored), ok
}

func (s *Service) ListManifests(owner principal.ID) []Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Manifest, 0)
	for _, item := range listManifestModels(&s.manifests, sortedKeys[model.Manifest]) {
		if item.Owner.Equal(owner) {
			items = append(items, manifestSnapshot(item))
		}
	}
	return items
}

func publishManifestModel(manifests *model.ManifestStore, blobs *model.BlobStore, manifest model.Manifest, nextID func(string) string, now time.Time) (model.Manifest, error) {
	manifest = normalizeManifestModel(manifest)
	if manifest.Owner.String() == "" {
		return model.Manifest{}, fmt.Errorf("manifest owner is required")
	}
	if manifest.Kind == "chunk-leaf" || manifest.Kind == "chunk-root" {
		if err := ValidateManifest(manifest); err != nil {
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
			if _, ok := manifests.Get(manifest.Owner, ref.ID); !ok {
				return model.Manifest{}, fmt.Errorf("manifest ref %q not found", ref.ID)
			}
		}
	}
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = now.UTC()
	}
	if err := manifests.Put(manifest); err != nil {
		return model.Manifest{}, err
	}
	return cloneManifestModel(manifest), nil
}

func getManifestModel(manifests *model.ManifestStore, owner principal.ID, id string) (model.Manifest, bool) {
	manifest, ok := manifests.Get(owner, id)
	return cloneManifestModel(manifest), ok
}

func listManifestModels(manifests *model.ManifestStore, keys func(map[string]model.Manifest) []string) []model.Manifest {
	out := make([]model.Manifest, 0, len(manifests.Items))
	for _, id := range keys(manifests.Items) {
		out = append(out, cloneManifestModel(manifests.Items[id]))
	}
	return out
}

func normalizeManifestModel(manifest model.Manifest) model.Manifest {
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

func cloneManifestModel(manifest model.Manifest) model.Manifest {
	manifest.Metadata = cloneMap(manifest.Metadata)
	manifest.Refs = append([]model.Ref(nil), manifest.Refs...)
	return manifest
}

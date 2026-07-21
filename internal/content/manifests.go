package content

import (
	model "ardents/internal/content/catalog"
	"fmt"
	"time"
)

func (s *Service) PublishManifest(manifest Manifest) (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, err := publishManifestModel(&s.manifests, &s.blobs, manifestModel(manifest), s.nextID)
	if err != nil {
		return Manifest{}, err
	}
	s.state = "ready"
	return manifestSnapshot(stored), s.saveLocked()
}

func (s *Service) GetManifest(id string) (Manifest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := getManifestModel(&s.manifests, id)
	return manifestSnapshot(stored), ok
}

func (s *Service) ListManifests() []Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := listManifestModels(&s.manifests, sortedKeys[model.Manifest])
	out := make([]Manifest, 0, len(items))
	for _, item := range items {
		out = append(out, manifestSnapshot(item))
	}
	return out
}

func publishManifestModel(manifests *model.ManifestStore, blobs *model.BlobStore, manifest model.Manifest, nextID func(string) string) (model.Manifest, error) {
	manifest = normalizeManifestModel(manifest)
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
			if _, ok := manifests.Get(ref.ID); !ok {
				return model.Manifest{}, fmt.Errorf("manifest ref %q not found", ref.ID)
			}
		}
	}
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = time.Now().UTC()
	}
	manifests.Put(manifest)
	return cloneManifestModel(manifest), nil
}

func getManifestModel(manifests *model.ManifestStore, id string) (model.Manifest, bool) {
	manifest, ok := manifests.Get(id)
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

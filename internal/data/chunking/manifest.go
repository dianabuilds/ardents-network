package chunking

import (
	"encoding/json"
	"fmt"

	model "ardents/internal/data/model"
	"ardents/internal/data/payload"
)

const (
	MaxLeafRefs = 512
	MaxRootRefs = 512
)

type ManifestSpec struct {
	Owner               string
	MediaType           string
	KeyID               string
	Access              string
	Retention           string
	TotalPlaintextBytes int64
	chunkCount          int
}

type ManifestPlan struct {
	Leaves []model.Manifest
	Root   model.Manifest
}

func Plan(chunkIDs []string, spec ManifestSpec) (ManifestPlan, error) {
	spec = normalizeSpec(spec)
	if err := validatePlanInput(chunkIDs, spec); err != nil {
		return ManifestPlan{}, err
	}
	spec.chunkCount = len(chunkIDs)
	leaves := make([]model.Manifest, 0, (len(chunkIDs)+MaxLeafRefs-1)/MaxLeafRefs)
	for start := 0; start < len(chunkIDs); start += MaxLeafRefs {
		end := start + MaxLeafRefs
		if end > len(chunkIDs) {
			end = len(chunkIDs)
		}
		leaf, err := makeManifest("chunk-leaf", "blob", chunkIDs[start:end], spec, start)
		if err != nil {
			return ManifestPlan{}, err
		}
		leaves = append(leaves, leaf)
	}
	if len(leaves) == 1 {
		return ManifestPlan{Leaves: leaves, Root: leaves[0]}, nil
	}
	leafIDs := make([]string, len(leaves))
	for index := range leaves {
		leafIDs[index] = leaves[index].ID
	}
	root, err := makeManifest("chunk-root", "manifest", leafIDs, spec, 0)
	if err != nil {
		return ManifestPlan{}, err
	}
	return ManifestPlan{Leaves: leaves, Root: root}, nil
}

func makeManifest(kind, refKind string, ids []string, spec ManifestSpec, start int) (model.Manifest, error) {
	refs := make([]model.Ref, len(ids))
	for index, id := range ids {
		refs[index] = model.Ref{Kind: refKind, ID: id}
	}
	manifest := model.Manifest{
		Kind: kind, Owner: spec.Owner, Access: spec.Access, Retention: spec.Retention,
		Encrypted: true, Refs: refs, Metadata: manifestMetadata(spec, start, len(ids)),
	}
	id, err := CanonicalManifestID(manifest)
	if err != nil {
		return model.Manifest{}, err
	}
	manifest.ID = id
	return manifest, nil
}

func CanonicalManifestID(manifest model.Manifest) (string, error) {
	metadata, err := canonicalMetadata(manifest.Metadata)
	if err != nil {
		return "", err
	}
	canonical, err := json.Marshal(struct {
		Kind, Owner, Access, Retention string
		Encrypted                      bool
		Refs                           []model.Ref
		Metadata                       chunkMetadata
	}{manifest.Kind, manifest.Owner, manifest.Access, manifest.Retention, manifest.Encrypted, manifest.Refs, metadata})
	if err != nil {
		return "", err
	}
	_, id, err := payload.DeriveIdentity(canonical)
	return id, err
}

func manifestMetadata(spec ManifestSpec, start, count int) map[string]any {
	return map[string]any{
		"format_version": uint32(1), "chunk_size": int64(PlaintextChunkSize),
		"chunk_start": int64(start), "chunk_count": int64(count),
		"total_chunk_count":     int64(spec.chunkCount),
		"total_plaintext_bytes": spec.TotalPlaintextBytes,
		"media_type":            spec.MediaType, "cipher": payload.AES256GCMCipher, "key_id": spec.KeyID,
	}
}

func normalizeSpec(spec ManifestSpec) ManifestSpec {
	if spec.Access == "" {
		spec.Access = "participants"
	}
	if spec.Retention == "" {
		spec.Retention = "durable"
	}
	return spec
}

func validatePlanInput(chunkIDs []string, spec ManifestSpec) error {
	if len(chunkIDs) == 0 || spec.Owner == "" || spec.MediaType == "" || spec.KeyID == "" || spec.TotalPlaintextBytes <= 0 {
		return fmt.Errorf("chunk manifest input is incomplete")
	}
	if len(chunkIDs) > MaxLeafRefs*MaxRootRefs {
		return fmt.Errorf("chunk manifest exceeds maximum shape")
	}
	for _, id := range chunkIDs {
		if id == "" {
			return fmt.Errorf("chunk manifest contains empty blob identity")
		}
	}
	return nil
}

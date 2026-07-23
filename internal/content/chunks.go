package content

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"
	"ardents/internal/identity/principal"
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

const (
	MaxLeafRefs = 512
	MaxRootRefs = 512
)

type ManifestSpec struct {
	Owner               principal.ID
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
		end := min(start+MaxLeafRefs, len(chunkIDs))
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
	}{manifest.Kind, manifest.Owner.String(), manifest.Access, manifest.Retention, manifest.Encrypted, manifest.Refs, metadata})
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
	if len(chunkIDs) == 0 || spec.Owner.String() == "" || spec.MediaType == "" || spec.KeyID == "" || spec.TotalPlaintextBytes <= 0 {
		return fmt.Errorf("chunk manifest input is incomplete")
	}
	if len(chunkIDs) > MaxLeafRefs*MaxRootRefs {
		return fmt.Errorf("chunk manifest exceeds maximum shape")
	}
	if slices.Contains(chunkIDs, "") {
		return fmt.Errorf("chunk manifest contains empty blob identity")
	}
	return nil
}

type ResolvedPlan struct {
	Root                model.Manifest
	Leaves              []model.Manifest
	ChunkIDs            []string
	TotalPlaintextBytes int64
}

func Resolve(root model.Manifest, lookup func(string) (model.Manifest, bool)) (ResolvedPlan, error) {
	if err := ValidateManifest(root); err != nil {
		return ResolvedPlan{}, err
	}
	if root.Kind == "chunk-leaf" {
		return resolvedLeaf(root)
	}
	if lookup == nil {
		return ResolvedPlan{}, fmt.Errorf("chunk manifest resolver is unavailable")
	}
	rootMetadata, err := canonicalMetadata(root.Metadata)
	if err != nil {
		return ResolvedPlan{}, err
	}
	plan := ResolvedPlan{Root: root, Leaves: make([]model.Manifest, 0, len(root.Refs)), TotalPlaintextBytes: rootMetadata.TotalPlaintextBytes}
	var nextStart int64
	for _, ref := range root.Refs {
		leaf, ok := lookup(ref.ID)
		if !ok {
			return ResolvedPlan{}, fmt.Errorf("chunk leaf manifest %q is unavailable", ref.ID)
		}
		if err := ValidateManifest(leaf); err != nil {
			return ResolvedPlan{}, err
		}
		metadata, err := canonicalMetadata(leaf.Metadata)
		if err != nil {
			return ResolvedPlan{}, err
		}
		sameContract, err := sameManifestContract(root, leaf)
		if err != nil {
			return ResolvedPlan{}, err
		}
		if leaf.Kind != "chunk-leaf" || leaf.ID != ref.ID || metadata.ChunkStart != nextStart || !sameContract {
			return ResolvedPlan{}, fmt.Errorf("chunk leaf manifest contract mismatch")
		}
		plan.Leaves = append(plan.Leaves, leaf)
		for _, chunkRef := range leaf.Refs {
			plan.ChunkIDs = append(plan.ChunkIDs, chunkRef.ID)
		}
		nextStart += metadata.ChunkCount
	}
	if nextStart != rootMetadata.TotalChunkCount {
		return ResolvedPlan{}, fmt.Errorf("chunk manifest tree is incomplete")
	}
	return plan, nil
}

func resolvedLeaf(leaf model.Manifest) (ResolvedPlan, error) {
	metadata, err := canonicalMetadata(leaf.Metadata)
	if err != nil {
		return ResolvedPlan{}, err
	}
	ids := make([]string, len(leaf.Refs))
	for index := range leaf.Refs {
		ids[index] = leaf.Refs[index].ID
	}
	return ResolvedPlan{Root: leaf, Leaves: []model.Manifest{leaf}, ChunkIDs: ids, TotalPlaintextBytes: metadata.TotalPlaintextBytes}, nil
}

func sameManifestContract(root, leaf model.Manifest) (bool, error) {
	rootMetadata, err := canonicalMetadata(root.Metadata)
	if err != nil {
		return false, err
	}
	leafMetadata, err := canonicalMetadata(leaf.Metadata)
	if err != nil {
		return false, err
	}
	return root.Owner == leaf.Owner && root.Access == leaf.Access && root.Retention == leaf.Retention &&
		rootMetadata.TotalChunkCount == leafMetadata.TotalChunkCount &&
		rootMetadata.TotalPlaintextBytes == leafMetadata.TotalPlaintextBytes &&
		rootMetadata.MediaType == leafMetadata.MediaType && rootMetadata.Cipher == leafMetadata.Cipher && rootMetadata.KeyID == leafMetadata.KeyID, nil
}

type chunkMetadata struct {
	FormatVersion       int64  `json:"format_version"`
	ChunkSize           int64  `json:"chunk_size"`
	ChunkStart          int64  `json:"chunk_start"`
	ChunkCount          int64  `json:"chunk_count"`
	TotalChunkCount     int64  `json:"total_chunk_count"`
	TotalPlaintextBytes int64  `json:"total_plaintext_bytes"`
	MediaType           string `json:"media_type"`
	Cipher              string `json:"cipher"`
	KeyID               string `json:"key_id"`
}

func ValidateManifest(manifest model.Manifest) error {
	if !manifest.Encrypted || manifest.Owner.String() == "" || manifest.Access == "" || manifest.Retention == "" {
		return fmt.Errorf("chunk manifest security metadata is incomplete")
	}
	metadata, err := canonicalMetadata(manifest.Metadata)
	if err != nil {
		return err
	}
	if err := validateMetadata(metadata); err != nil {
		return err
	}
	if err := validateRefs(manifest, metadata); err != nil {
		return err
	}
	expected, err := CanonicalManifestID(manifest)
	if err != nil {
		return err
	}
	if manifest.ID != expected {
		return fmt.Errorf("chunk manifest content identity mismatch")
	}
	return nil
}

func canonicalMetadata(values map[string]any) (chunkMetadata, error) {
	allowed := map[string]bool{
		"format_version": true, "chunk_size": true, "chunk_start": true,
		"chunk_count": true, "total_chunk_count": true, "total_plaintext_bytes": true,
		"media_type": true, "cipher": true, "key_id": true,
	}
	for key := range values {
		if !allowed[key] {
			return chunkMetadata{}, fmt.Errorf("chunk manifest metadata field is unsupported")
		}
	}
	metadata := chunkMetadata{
		FormatVersion: numeric(values["format_version"]), ChunkSize: numeric(values["chunk_size"]),
		ChunkStart: numeric(values["chunk_start"]), ChunkCount: numeric(values["chunk_count"]),
		TotalChunkCount:     numeric(values["total_chunk_count"]),
		TotalPlaintextBytes: numeric(values["total_plaintext_bytes"]),
		MediaType:           text(values["media_type"]), Cipher: text(values["cipher"]), KeyID: text(values["key_id"]),
	}
	if metadata.FormatVersion < 0 || metadata.ChunkSize < 0 || metadata.ChunkStart < 0 || metadata.ChunkCount < 0 || metadata.TotalChunkCount < 0 || metadata.TotalPlaintextBytes < 0 {
		return chunkMetadata{}, fmt.Errorf("chunk manifest numeric metadata is invalid")
	}
	return metadata, nil
}

func validateMetadata(metadata chunkMetadata) error {
	if metadata.FormatVersion != 1 || metadata.ChunkSize != PlaintextChunkSize || metadata.TotalPlaintextBytes <= 0 || metadata.TotalChunkCount <= 0 {
		return fmt.Errorf("chunk manifest format metadata is invalid")
	}
	if metadata.MediaType == "" || metadata.Cipher != payload.AES256GCMCipher || metadata.KeyID == "" {
		return fmt.Errorf("chunk manifest encryption metadata is invalid")
	}
	return nil
}

func validateRefs(manifest model.Manifest, metadata chunkMetadata) error {
	limit, refKind := MaxLeafRefs, "blob"
	if manifest.Kind == "chunk-root" {
		limit, refKind = MaxRootRefs, "manifest"
	} else if manifest.Kind != "chunk-leaf" {
		return fmt.Errorf("chunk manifest kind is invalid")
	}
	if len(manifest.Refs) == 0 || len(manifest.Refs) > limit || metadata.ChunkCount != int64(len(manifest.Refs)) {
		return fmt.Errorf("chunk manifest shape is invalid")
	}
	if manifest.Kind == "chunk-leaf" && metadata.ChunkStart+metadata.ChunkCount > metadata.TotalChunkCount {
		return fmt.Errorf("chunk manifest range is invalid")
	}
	for _, ref := range manifest.Refs {
		if ref.Kind != refKind || ref.ID == "" {
			return fmt.Errorf("chunk manifest ref is invalid")
		}
	}
	return nil
}

func numeric(value any) int64 {
	switch number := value.(type) {
	case int:
		return int64(number)
	case int64:
		return number
	case uint32:
		return int64(number)
	case float64:
		if number >= 0 && number <= math.MaxInt64 && math.Trunc(number) == number {
			return int64(number)
		}
	case json.Number:
		parsed, err := number.Int64()
		if err == nil {
			return parsed
		}
	}
	return -1
}

func text(value any) string {
	valueString, _ := value.(string)
	return valueString
}

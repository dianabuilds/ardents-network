package chunking

import (
	"encoding/json"
	"fmt"
	"math"

	model "ardents/internal/data/model"
	"ardents/internal/data/payload"
)

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
	if !manifest.Encrypted || manifest.Owner == "" || manifest.Access == "" || manifest.Retention == "" {
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
		parsed, _ := number.Int64()
		return parsed
	}
	return -1
}

func text(value any) string {
	valueString, _ := value.(string)
	return valueString
}

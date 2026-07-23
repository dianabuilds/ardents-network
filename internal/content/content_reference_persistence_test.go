package content

import (
	"testing"
	"time"

	"ardents/internal/content/catalog"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestContentStateRejectsObsoleteAndMismatchedBlobIdentities(t *testing.T) {
	reference := testContentReference(t, "persisted-reference")
	other := testContentReference(t, "other-reference")
	base := func(blob any, key string) map[string]any {
		return map[string]any{
			"version": contentSchemaVersion,
			"objects": map[string]any{}, "blobs": map[string]any{key: blob},
			"sources": map[string]any{}, "manifests": map[string]any{},
			"blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": []any{}},
		}
	}
	tests := map[string]map[string]any{
		"obsolete id":         base(map[string]any{"id": reference.String(), "media_type": "application/octet-stream", "encrypted": false, "expires_at": time.Time{}, "created_at": time.Now().UTC()}, reference.String()),
		"obsolete cid":        base(map[string]any{"cid": reference.String(), "media_type": "application/octet-stream", "encrypted": false, "expires_at": time.Time{}, "created_at": time.Now().UTC()}, reference.String()),
		"malformed reference": base(map[string]any{"reference": "not-a-cid", "media_type": "application/octet-stream", "encrypted": false, "expires_at": time.Time{}, "created_at": time.Now().UTC()}, reference.String()),
		"map key mismatch":    base(Blob{Reference: other, MediaType: "application/octet-stream", CreatedAt: time.Now().UTC()}, reference.String()),
	}
	for name, snapshot := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.SaveJSON(contentPath(dir), "data", "snapshot", snapshot))
			service := NewInDir(dir)
			require.Error(t, service.Load())
			require.Empty(t, service.ListBlobs())
		})
	}
}

func TestContentLoadRollbackPreservesLiveStateWhenOwnershipIsInvalid(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	existing, err := service.StoreBlob(Blob{MediaType: "application/octet-stream"}, []byte("existing"))
	require.NoError(t, err)

	replacementReference := testContentReference(t, "replacement")
	unknownReference := testContentReference(t, "unknown-owner-target")
	owner := contentTestPrincipal(t, 77)
	snapshot := persistedContent{
		Version: contentSchemaVersion,
		Objects: map[string]catalog.Object{},
		Blobs: map[string]catalog.Blob{replacementReference.String(): {
			Reference: replacementReference, MediaType: "application/octet-stream", CreatedAt: time.Now().UTC(),
		}},
		Sources: map[string][]catalog.BlobSourceRecord{}, Manifests: map[string]catalog.Manifest{},
		BlobOwnership: persistedBlobOwnership{Version: blobOwnershipVersion, Bindings: []catalog.BlobOwnerBinding{{
			Owner: owner, Reference: unknownReference, CreatedAt: time.Now().UTC(),
		}}},
	}
	require.NoError(t, storage.SaveJSON(contentPath(dir), "data", "snapshot", snapshot))
	require.Error(t, service.Load())

	loaded, ok := service.GetBlob(existing.Reference.String())
	require.True(t, ok)
	require.Equal(t, existing, loaded)
	_, ok = service.GetBlob(replacementReference.String())
	require.False(t, ok)
}

func TestContentStateRejectsObsoleteSchemaAndPartialCollections(t *testing.T) {
	for name, snapshot := range map[string]map[string]any{
		"schema v1": {"version": 1, "objects": map[string]any{}, "blobs": map[string]any{}, "sources": map[string]any{}, "manifests": map[string]any{}, "blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": []any{}}},
		"partial":   {"version": contentSchemaVersion, "objects": map[string]any{}, "blobs": map[string]any{}, "sources": map[string]any{}, "blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": []any{}}},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.SaveJSON(contentPath(dir), "data", "snapshot", snapshot))
			require.Error(t, NewInDir(dir).Load())
		})
	}
}

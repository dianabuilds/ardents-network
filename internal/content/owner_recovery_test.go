package content

import (
	"ardents/internal/storage"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestObjectAndManifestRequireCanonicalPrincipalOwner(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())

	_, err := service.PublishObject(Object{Type: "document"})
	require.ErrorContains(t, err, "object owner is required")
	_, err = service.PublishManifest(Manifest{Kind: "blob-set"})
	require.ErrorContains(t, err, "manifest owner is required")
}

func TestLoadRejectsUnknownContentSchemaAndMalformedTypedOwners(t *testing.T) {
	now := time.Now().UTC()
	tests := map[string]map[string]any{
		"unknown schema": {
			"version": 99, "objects": map[string]any{}, "blobs": map[string]any{},
			"sources": map[string]any{}, "manifests": map[string]any{},
			"blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": []any{}},
		},
		"malformed object owner": {
			"version": contentSchemaVersion,
			"objects": map[string]any{"object": map[string]any{"id": "object", "type": "document", "owner": "principal.local", "created_at": now}},
			"blobs":   map[string]any{}, "sources": map[string]any{}, "manifests": map[string]any{},
			"blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": []any{}},
		},
		"malformed manifest owner": {
			"version": contentSchemaVersion,
			"objects": map[string]any{}, "blobs": map[string]any{}, "sources": map[string]any{},
			"manifests":      map[string]any{"manifest": map[string]any{"id": "manifest", "kind": "blob-set", "owner": "node", "created_at": now}},
			"blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": []any{}},
		},
	}
	for name, snapshot := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.SaveJSON(contentPath(dir), "data", "snapshot", snapshot))
			require.Error(t, NewInDir(dir).Load())
		})
	}
}

func TestDropOwnerBindingPreservesSharedThenReclaimsUnreferencedPayload(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	alice := contentTestOwner(0x51)
	bob := contentTestOwner(0x52)
	aliceBlob, err := service.StoreBlobForOwner(alice, Blob{MediaType: "text/plain"}, []byte("shared"))
	require.NoError(t, err)
	_, err = service.StoreBlobForOwner(bob, Blob{MediaType: "text/plain"}, []byte("shared"))
	require.NoError(t, err)

	_, err = service.DropBlobForOwner(alice, aliceBlob.Reference.String())
	require.NoError(t, err)
	require.False(t, service.HasBlobOwner(alice, aliceBlob.Reference.String()))
	require.True(t, service.HasBlobOwner(bob, aliceBlob.Reference.String()))
	_, err = service.GetBlobPayloadForOwner(bob, aliceBlob.Reference.String())
	require.NoError(t, err)

	dropped, err := service.DropBlobForOwner(bob, aliceBlob.Reference.String())
	require.NoError(t, err)
	require.Equal(t, "deleted", dropped.State)
	_, statErr := os.Stat(service.payloadPath(aliceBlob.Reference.String()))
	require.ErrorIs(t, statErr, os.ErrNotExist)

	restarted := NewInDir(dir)
	require.NoError(t, restarted.Load())
	require.False(t, restarted.HasBlobOwner(alice, aliceBlob.Reference.String()))
	require.False(t, restarted.HasBlobOwner(bob, aliceBlob.Reference.String()))
}

func TestDropOwnerBindingKeepsReferencedOrIndependentlyRetainedPayload(t *testing.T) {
	owner := contentTestOwner(0x53)
	for name, retain := range map[string]func(*Service, string) error{
		"object reference": func(service *Service, id string) error {
			_, err := service.PublishObject(Object{Type: "document", Owner: owner, BlobRefs: []Ref{{Kind: "blob", ID: id}}})
			return err
		},
		"manifest reference": func(service *Service, id string) error {
			_, err := service.PublishManifest(Manifest{Kind: "blob-set", Owner: owner, Refs: []Ref{{Kind: "blob", ID: id}}})
			return err
		},
		"pin": func(service *Service, id string) error {
			_, err := service.PinBlob(id)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewInDir(t.TempDir())
			require.NoError(t, service.Load())
			blob, err := service.StoreBlobForOwner(owner, Blob{MediaType: "text/plain"}, []byte(name))
			require.NoError(t, err)
			require.NoError(t, retain(service, blob.Reference.String()))
			_, err = service.DropBlobForOwner(owner, blob.Reference.String())
			require.NoError(t, err)
			_, err = service.GetBlobPayload(blob.Reference.String())
			require.NoError(t, err)
		})
	}
}

func TestDropOwnerBindingRollsBackOnCatalogueFailure(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	owner := contentTestOwner(0x54)
	blob, err := service.StoreBlobForOwner(owner, Blob{MediaType: "text/plain"}, []byte("rollback"))
	require.NoError(t, err)
	breakMetadataPersistence(t, service, dir)

	_, err = service.DropBlobForOwner(owner, blob.Reference.String())
	require.Error(t, err)
	require.True(t, service.HasBlobOwner(owner, blob.Reference.String()))
	raw, err := service.GetBlobPayloadForOwner(owner, blob.Reference.String())
	require.NoError(t, err)
	require.Equal(t, []byte("rollback"), raw)
}

func TestRemoteFetchNeverCreatesOwnerBindingAndCanRefillExistingBinding(t *testing.T) {
	origin := NewInDir(t.TempDir())
	require.NoError(t, origin.Load())
	remote, err := origin.StoreBlob(Blob{MediaType: "text/plain"}, []byte("remote"))
	require.NoError(t, err)

	withoutBinding := NewInDir(t.TempDir())
	require.NoError(t, withoutBinding.Load())
	_, err = withoutBinding.FetchBlob(remote.Reference.String(), origin)
	require.NoError(t, err)
	require.False(t, withoutBinding.HasBlobOwner(contentTestOwner(0x55), remote.Reference.String()))

	owner := contentTestOwner(0x56)
	withBinding := NewInDir(t.TempDir())
	require.NoError(t, withBinding.Load())
	owned, err := withBinding.StoreBlobForOwner(owner, Blob{MediaType: "text/plain"}, []byte("remote"))
	require.NoError(t, err)
	_, err = withBinding.DropBlob(owned.Reference.String())
	require.NoError(t, err)
	require.True(t, withBinding.HasBlobOwner(owner, owned.Reference.String()))
	_, err = withBinding.FetchBlob(owned.Reference.String(), origin)
	require.NoError(t, err)
	raw, err := withBinding.GetBlobPayloadForOwner(owner, owned.Reference.String())
	require.NoError(t, err)
	require.Equal(t, []byte("remote"), raw)
}

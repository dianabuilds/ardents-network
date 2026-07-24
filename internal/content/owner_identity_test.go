package content

import (
	"os"
	"path/filepath"
	"testing"

	"ardents/internal/content/catalog"
	"ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestObjectAndManifestIdentityIsOwnerQualified(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	alice := contentTestOwner(0x61)
	bob := contentTestOwner(0x62)

	for _, owner := range []principal.ID{alice, bob} {
		_, err := service.PublishObject(Object{ID: "shared", Owner: owner, Type: owner.String()})
		require.NoError(t, err)
		_, err = service.PublishManifest(Manifest{ID: "shared", Owner: owner, Kind: "blob-set", Metadata: map[string]any{"owner": owner.String()}})
		require.NoError(t, err)
	}

	aliceObject, ok := service.GetObject(alice, "shared")
	require.True(t, ok)
	require.Equal(t, alice.String(), aliceObject.Type)
	bobObject, ok := service.GetObject(bob, "shared")
	require.True(t, ok)
	require.Equal(t, bob.String(), bobObject.Type)

	aliceManifest, ok := service.GetManifest(alice, "shared")
	require.True(t, ok)
	require.Equal(t, alice.String(), aliceManifest.Metadata["owner"])
	bobManifest, ok := service.GetManifest(bob, "shared")
	require.True(t, ok)
	require.Equal(t, bob.String(), bobManifest.Metadata["owner"])

	require.Len(t, service.ListObjects(alice), 1)
	require.Len(t, service.ListManifests(bob), 1)
}

func TestChunkRollbackDeletesOnlyThePublishingOwnersManifest(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	alice := contentTestOwner(0x66)
	bob := contentTestOwner(0x67)
	for _, owner := range []principal.ID{alice, bob} {
		_, err := service.PublishManifest(Manifest{ID: "shared", Owner: owner, Kind: "blob-set"})
		require.NoError(t, err)
	}

	require.NoError(t, service.rollbackChunkedPayload(bob, nil, []string{"shared"}))
	_, ok := service.GetManifest(alice, "shared")
	require.True(t, ok)
	_, ok = service.GetManifest(bob, "shared")
	require.False(t, ok)
}

func TestContentSchemaV2MigrationRekeysOwnerQualifiedRecords(t *testing.T) {
	dir := t.TempDir()
	path := contentPath(dir)
	alice := contentTestOwner(0x63)
	bob := contentTestOwner(0x65)
	legacy := persistedContent{
		Version: 2,
		Objects: map[string]catalog.Object{
			"alice-object": {ID: "alice-object", Owner: alice, Type: "document"},
			"bob-object":   {ID: "bob-object", Owner: bob, Type: "message"},
		},
		Blobs:   map[string]catalog.Blob{},
		Sources: map[string][]catalog.BlobSourceRecord{},
		Manifests: map[string]catalog.Manifest{
			"alice-manifest": {ID: "alice-manifest", Owner: alice, Kind: "blob-set"},
			"bob-manifest":   {ID: "bob-manifest", Owner: bob, Kind: "blob-set"},
		},
		BlobOwnership: persistedBlobOwnership{Version: blobOwnershipVersion, Bindings: []catalog.BlobOwnerBinding{}},
	}
	require.NoError(t, saveContent(path, legacy))
	untracked := filepath.Join(dir, "blobs", "legacy-untracked.blob")
	require.NoError(t, os.MkdirAll(filepath.Dir(untracked), 0o700))
	require.NoError(t, os.WriteFile(untracked, []byte("rollback payload"), 0o600))

	service := NewInDir(dir)
	require.NoError(t, service.Load())
	_, err := os.Stat(untracked)
	require.NoError(t, err, "migration commit must defer destructive payload reconciliation")
	_, ok := service.GetObject(alice, "alice-object")
	require.True(t, ok)
	_, ok = service.GetManifest(alice, "alice-manifest")
	require.True(t, ok)
	_, ok = service.GetObject(bob, "bob-object")
	require.True(t, ok)
	_, ok = service.GetManifest(bob, "bob-manifest")
	require.True(t, ok)

	var upgraded persistedContent
	found, err := loadContent(path, &upgraded)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint32(3), upgraded.Version)
	require.Contains(t, upgraded.Objects, catalog.RecordStorageKey(alice, "alice-object"))
	require.Contains(t, upgraded.Objects, catalog.RecordStorageKey(bob, "bob-object"))
	require.Contains(t, upgraded.Manifests, catalog.RecordStorageKey(alice, "alice-manifest"))
	require.Contains(t, upgraded.Manifests, catalog.RecordStorageKey(bob, "bob-manifest"))

	require.NoError(t, NewInDir(dir).Load())
	_, err = os.Stat(untracked)
	require.ErrorIs(t, err, os.ErrNotExist, "ordinary v3 load should resume payload reconciliation")
}

func TestContentSchemaV2MigrationRejectsMismatchedLegacyKey(t *testing.T) {
	dir := t.TempDir()
	path := contentPath(dir)
	alice := contentTestOwner(0x64)
	legacy := persistedContent{
		Version: 2,
		Objects: map[string]catalog.Object{
			"wrong-key": {ID: "embedded-id", Owner: alice, Type: "first"},
		},
		Blobs:         map[string]catalog.Blob{},
		Sources:       map[string][]catalog.BlobSourceRecord{},
		Manifests:     map[string]catalog.Manifest{},
		BlobOwnership: persistedBlobOwnership{Version: blobOwnershipVersion, Bindings: []catalog.BlobOwnerBinding{}},
	}
	require.NoError(t, saveContent(path, legacy))
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	err = NewInDir(dir).Load()
	require.ErrorContains(t, err, "legacy object identity is invalid")
	after, readErr := os.ReadFile(filepath.Clean(path))
	require.NoError(t, readErr)
	require.Equal(t, before, after)
}

func TestContentSchemaV2MigrationRejectsCrossOwnerManifestReference(t *testing.T) {
	dir := t.TempDir()
	path := contentPath(dir)
	alice := contentTestOwner(0x66)
	bob := contentTestOwner(0x67)
	legacy := persistedContent{
		Version: 2,
		Objects: map[string]catalog.Object{},
		Blobs:   map[string]catalog.Blob{},
		Sources: map[string][]catalog.BlobSourceRecord{},
		Manifests: map[string]catalog.Manifest{
			"root": {
				ID: "root", Owner: alice, Kind: "blob-set",
				Refs: []catalog.Ref{{Kind: "manifest", ID: "leaf"}},
			},
			"leaf": {ID: "leaf", Owner: bob, Kind: "blob-set"},
		},
		BlobOwnership: persistedBlobOwnership{Version: blobOwnershipVersion, Bindings: []catalog.BlobOwnerBinding{}},
	}
	require.NoError(t, saveContent(path, legacy))
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	err = NewInDir(dir).Load()
	require.ErrorContains(t, err, "same owner")
	after, readErr := os.ReadFile(filepath.Clean(path))
	require.NoError(t, readErr)
	require.Equal(t, before, after)
}

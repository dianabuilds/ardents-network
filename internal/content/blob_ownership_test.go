package content

import (
	"ardents/internal/content/payload"
	"ardents/internal/identity/principal"
	"ardents/internal/storage"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOwnedBlobPutCommitsBindingAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	owner := contentTestPrincipal(t, 1)
	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	service := NewInDirWithConfig(dir, Config{Now: func() time.Time { return now }})
	require.NoError(t, service.Load())

	stored, err := service.StoreBlobForOwner(owner, Blob{MediaType: "text/plain"}, []byte("owned"))
	require.NoError(t, err)
	require.True(t, service.HasBlobOwner(owner, stored.ID))
	require.Equal(t, now, stored.CreatedAt)
	require.Equal(t, now, service.blobOwners.Snapshot()[0].CreatedAt)

	restarted := NewInDir(dir)
	require.NoError(t, restarted.Load())
	loaded, found := restarted.GetBlobForOwner(owner, stored.ID)
	require.True(t, found)
	require.Equal(t, stored, loaded)
	raw, err := restarted.GetBlobPayloadForOwner(owner, stored.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("owned"), raw)
}

func TestOwnedBlobIsDeduplicatedWithoutGrantingSiblingOwner(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	alice := contentTestPrincipal(t, 2)
	bob := contentTestPrincipal(t, 3)
	unknown := contentTestPrincipal(t, 4)

	aliceBlob, err := service.StoreBlobForOwner(alice, Blob{MediaType: "text/plain"}, []byte("same bytes"))
	require.NoError(t, err)
	_, found := service.GetBlobForOwner(bob, aliceBlob.ID)
	require.False(t, found)
	_, err = service.GetBlobPayloadForOwner(bob, aliceBlob.ID)
	require.ErrorIs(t, err, ErrBlobNotFound)

	bobBlob, err := service.StoreBlobForOwner(bob, Blob{MediaType: "text/plain"}, []byte("same bytes"))
	require.NoError(t, err)
	require.Equal(t, aliceBlob.ID, bobBlob.ID)
	require.Len(t, service.ListBlobs(), 1)
	require.True(t, service.HasBlobOwner(alice, aliceBlob.ID))
	require.True(t, service.HasBlobOwner(bob, aliceBlob.ID))
	require.False(t, service.HasBlobOwner(unknown, aliceBlob.ID))
}

func TestOwnedBlobSupportsEmptyPayload(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	owner := contentTestPrincipal(t, 5)

	stored, err := service.StoreBlobForOwner(owner, Blob{MediaType: "application/octet-stream"}, []byte{})
	require.NoError(t, err)
	_, expectedReference, err := payload.DeriveIdentity(nil)
	require.NoError(t, err)
	require.Equal(t, expectedReference, stored.ID)
	raw, err := service.GetBlobPayloadForOwner(owner, stored.ID)
	require.NoError(t, err)
	require.Empty(t, raw)
}

func TestOwnedBlobRollsBackNewPayloadAndBindingWhenCatalogueCommitFails(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	owner := contentTestPrincipal(t, 6)
	_, reference, err := payload.DeriveIdentity([]byte("uncommitted"))
	require.NoError(t, err)
	breakMetadataPersistence(t, service, dir)

	_, err = service.StoreBlobForOwner(owner, Blob{MediaType: "text/plain"}, []byte("uncommitted"))
	require.Error(t, err)
	require.False(t, service.HasBlobOwner(owner, reference))
	_, found := service.GetBlob(reference)
	require.False(t, found)
	_, statErr := os.Stat(service.payloadPath(reference))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestOwnedBlobRollsBackOnlyNewBindingForExistingPayload(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	first := contentTestPrincipal(t, 7)
	second := contentTestPrincipal(t, 8)
	stored, err := service.StoreBlobForOwner(first, Blob{MediaType: "text/plain"}, []byte("preserved"))
	require.NoError(t, err)
	breakMetadataPersistence(t, service, dir)

	_, err = service.StoreBlobForOwner(second, Blob{MediaType: "text/plain"}, []byte("preserved"))
	require.Error(t, err)
	require.True(t, service.HasBlobOwner(first, stored.ID))
	require.False(t, service.HasBlobOwner(second, stored.ID))
	raw, err := service.GetBlobPayloadForOwner(first, stored.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("preserved"), raw)
}

func TestOwnedBlobConcurrentIdempotentPutCreatesOneBinding(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	owner := contentTestPrincipal(t, 9)

	const writers = 16
	errs := make(chan error, writers)
	var wait sync.WaitGroup
	for range writers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.StoreBlobForOwner(owner, Blob{MediaType: "text/plain"}, []byte("concurrent"))
			errs <- err
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Len(t, service.ListBlobs(), 1)
	require.Equal(t, 1, service.blobOwners.Count())
}

func TestLoadRejectsUnknownBlobOwnershipVersion(t *testing.T) {
	dir := t.TempDir()
	path := contentPath(dir)
	require.NoError(t, storage.SaveJSON(path, "data", "snapshot", map[string]any{
		"version": contentSchemaVersion,
		"objects": map[string]any{}, "blobs": map[string]any{},
		"sources": map[string]any{}, "manifests": map[string]any{},
		"blob_ownership": map[string]any{"version": 99, "bindings": []any{}},
	}))

	err := NewInDir(dir).Load()
	require.ErrorContains(t, err, "unsupported blob ownership version")
}

func TestLoadRejectsMalformedAndDuplicateBlobOwnerBindings(t *testing.T) {
	owner := contentTestPrincipal(t, 10)
	createdAt := time.Now().UTC()
	validBlob := Blob{ID: "ref", CID: "ref", CreatedAt: createdAt}
	tests := map[string][]map[string]any{
		"malformed principal": {{"owner": "p1_invalid", "reference": "ref", "created_at": createdAt}},
		"unknown reference":   {{"owner": owner.String(), "reference": "missing", "created_at": createdAt}},
		"duplicate": {
			{"owner": owner.String(), "reference": "ref", "created_at": createdAt},
			{"owner": owner.String(), "reference": "ref", "created_at": createdAt},
		},
	}
	for name, bindings := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.SaveJSON(contentPath(dir), "data", "snapshot", map[string]any{
				"version": contentSchemaVersion,
				"objects": map[string]any{}, "blobs": map[string]Blob{"ref": validBlob},
				"sources": map[string]any{}, "manifests": map[string]any{},
				"blob_ownership": map[string]any{"version": blobOwnershipVersion, "bindings": bindings},
			}))
			require.Error(t, NewInDir(dir).Load())
		})
	}
}

func TestStartupReclaimsInstalledPayloadWithoutInventingBinding(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	_, reference, err := payload.DeriveIdentity([]byte("orphan"))
	require.NoError(t, err)
	require.NoError(t, storage.AtomicWritePrivateFile(service.payloadPath(reference), []byte("orphan")))

	restarted := NewInDir(dir)
	require.NoError(t, restarted.Load())
	_, statErr := os.Stat(filepath.Join(dir, "blobs", reference+".blob"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
	require.False(t, restarted.HasBlobOwner(contentTestPrincipal(t, 11), reference))
}

func contentTestPrincipal(t *testing.T, marker byte) principal.ID {
	t.Helper()
	id := contentTestOwner(marker)
	require.NotEmpty(t, id.String())
	return id
}

func contentTestOwner(marker byte) principal.ID {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = marker
	}
	key := ed25519.NewKeyFromSeed(seed)
	id, err := principal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	if err != nil {
		panic("derive fixed test content owner")
	}
	return id
}

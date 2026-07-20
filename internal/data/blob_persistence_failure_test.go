package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreBlobRollsBackPayloadWhenMetadataCommitFails(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	breakMetadataPersistence(t, service, dir)

	_, err := service.StoreBlob(Blob{MediaType: "text/plain"}, []byte("uncommitted"))
	require.Error(t, err)
	require.Empty(t, service.ListBlobs())
	require.Empty(t, blobDirectoryEntries(t, dir))
}

func TestDropBlobRestoresPayloadWhenMetadataCommitFails(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	blob, err := service.StoreBlob(Blob{MediaType: "text/plain"}, []byte("keep on failure"))
	require.NoError(t, err)
	breakMetadataPersistence(t, service, dir)

	_, err = service.DropBlob(blob.ID)
	require.Error(t, err)
	current, found := service.GetBlob(blob.ID)
	require.True(t, found)
	require.Equal(t, "available-local", current.State)
	payload, err := service.GetBlobPayload(blob.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("keep on failure"), payload)
	requireNoPrivatePayloads(t, dir)
}

func TestPruneExpiredRestoresPayloadWhenMetadataCommitFails(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	blob, err := service.StoreBlob(Blob{MediaType: "text/plain"}, []byte("retain on failure"))
	require.NoError(t, err)
	expiresAt := time.Now().UTC().Add(-time.Minute)
	_, err = service.RetainBlob(blob.ID, expiresAt)
	require.NoError(t, err)
	breakMetadataPersistence(t, service, dir)

	_, err = service.PruneExpired(time.Now().UTC())
	require.Error(t, err)
	current, found := service.GetBlob(blob.ID)
	require.True(t, found)
	require.Equal(t, "retained-temporary", current.State)
	payload, err := service.GetBlobPayload(blob.ID)
	require.NoError(t, err)
	require.Equal(t, []byte("retain on failure"), payload)
	requireNoPrivatePayloads(t, dir)
}

func breakMetadataPersistence(t *testing.T, service *Service, dir string) {
	t.Helper()
	brokenPath := filepath.Join(dir, "broken-database")
	require.NoError(t, os.Mkdir(brokenPath, 0o700))
	service.path = brokenPath
}

func blobDirectoryEntries(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "blobs"))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return entries
}

func requireNoPrivatePayloads(t *testing.T, dir string) {
	t.Helper()
	for _, entry := range blobDirectoryEntries(t, dir) {
		require.False(t, strings.HasPrefix(entry.Name(), ".ardents-private-"))
	}
}

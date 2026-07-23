package content

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreChunkedPayloadEncryptsChunksAndPublishesManifest(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	key := bytes.Repeat([]byte{0x42}, 32)
	plaintext := bytes.Repeat([]byte("payload"), 10000)

	result, err := service.StoreChunkedPayload(context.Background(), ChunkedPayloadSpec{
		Owner: contentTestOwner(0x34), MediaType: "application/octet-stream", KeyID: "key-1",
		Access: "participants", Retention: "durable",
	}, bytes.NewReader(plaintext), key)
	require.NoError(t, err)
	require.Equal(t, 2, result.ChunkCount)
	require.Equal(t, int64(len(plaintext)), result.TotalPlaintextBytes)
	require.Equal(t, "chunk-leaf", result.Root.Kind)
	require.Len(t, result.Root.Refs, 2)

	var reconstructed []byte
	for _, ref := range result.Root.Refs {
		blob, ok := service.GetBlob(ref.ID)
		require.True(t, ok)
		require.True(t, blob.Encrypted)
		require.Equal(t, "durable", blob.Retention)
		chunk, err := service.DecryptBlobPayload(ref.ID, key)
		require.NoError(t, err)
		reconstructed = append(reconstructed, chunk...)
	}
	require.Equal(t, plaintext, reconstructed)
	stored, ok := service.GetManifest(result.Root.ID)
	require.True(t, ok)
	require.Equal(t, result.Root, stored)
}

func TestLoadCleansOrphanChunkStagingAndFinalizesReferencedChunk(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	key := bytes.Repeat([]byte{0x51}, 32)
	orphan, err := service.storeStagedChunk(Blob{MediaType: "application/octet-stream", Retention: "staging"}, []byte("orphan"), key, "")
	require.NoError(t, err)
	referenced, err := service.storeStagedChunk(Blob{MediaType: "application/octet-stream", Retention: "staging"}, []byte("referenced"), key, "")
	require.NoError(t, err)
	_, err = service.PublishManifest(Manifest{Kind: "blob-set", Owner: contentTestOwner(0x34), Encrypted: true, Retention: "durable", Refs: []Ref{{Kind: "blob", ID: referenced.Reference.String()}}})
	require.NoError(t, err)

	reloaded := NewInDir(dir)
	require.NoError(t, reloaded.Load())
	_, exists := reloaded.GetBlob(orphan.Reference.String())
	require.False(t, exists)
	kept, exists := reloaded.GetBlob(referenced.Reference.String())
	require.True(t, exists)
	require.Equal(t, "durable", kept.Retention)
}

func TestStoreChunkedPayloadRollsBackOnLocalStoragePressure(t *testing.T) {
	service := NewInDirWithConfig(t.TempDir(), Config{MaxLocalStorageBytes: 1024})
	require.NoError(t, service.Load())
	key := bytes.Repeat([]byte{0x52}, 32)
	_, err := service.StoreChunkedPayload(context.Background(), ChunkedPayloadSpec{
		Owner: contentTestOwner(0x34), MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(bytes.Repeat([]byte("x"), PlaintextChunkSize)), key)
	require.ErrorContains(t, err, "storage capacity")
	require.Empty(t, service.ListBlobs())
	require.Empty(t, service.ListManifests())
}

func TestStoreChunkedPayloadRollsBackOnPayloadWriteFailure(t *testing.T) {
	dir := t.TempDir()
	service := NewInDir(dir)
	require.NoError(t, service.Load())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "blobs"), []byte("not a directory"), 0o600))

	key := bytes.Repeat([]byte{0x54}, 32)
	_, err := service.StoreChunkedPayload(context.Background(), ChunkedPayloadSpec{
		Owner: contentTestOwner(0x34), MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(bytes.Repeat([]byte("x"), PlaintextChunkSize)), key)
	require.Error(t, err)
	require.Empty(t, service.ListBlobs())
	require.Empty(t, service.ListManifests())
}

func TestLoadRemovesUntrackedChunkFileLeftByInterruptedCheckpoint(t *testing.T) {
	dir := t.TempDir()
	blobDir := filepath.Join(dir, "blobs")
	require.NoError(t, os.MkdirAll(blobDir, 0o700))
	orphanPath := filepath.Join(blobDir, "untracked.blob")
	require.NoError(t, os.WriteFile(orphanPath, []byte("partial ciphertext"), 0o600))

	service := NewInDir(dir)
	require.NoError(t, service.Load())
	_, err := os.Stat(orphanPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestStagedChunkIsNotReportedOrServedAsAvailable(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	key := bytes.Repeat([]byte{0x53}, 32)
	blob, err := service.storeStagedChunk(Blob{MediaType: "application/octet-stream", Retention: "staging"}, []byte("partial"), key, "")
	require.NoError(t, err)
	stored, ok := service.GetBlob(blob.Reference.String())
	require.True(t, ok)
	require.Equal(t, "staging", stored.State)
	require.Zero(t, service.InventorySnapshot().AvailableForResend)
	_, err = service.GetBlobPayload(blob.Reference.String())
	require.ErrorContains(t, err, "not locally available")
}

func TestStoreChunkedPayloadCancellationAndReadFailureRollback(t *testing.T) {
	service := NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	key := bytes.Repeat([]byte{0x24}, 32)
	spec := ChunkedPayloadSpec{Owner: contentTestOwner(0x34), MediaType: "application/octet-stream", KeyID: "key-1"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.StoreChunkedPayload(ctx, spec, bytes.NewReader([]byte("payload")), key)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, service.ListBlobs())

	_, err = service.StoreChunkedPayload(context.Background(), spec, &failingChunkReader{
		remaining: bytes.Repeat([]byte("x"), PlaintextChunkSize),
	}, key)
	require.ErrorContains(t, err, "injected read failure")
	require.Empty(t, service.ListBlobs())
	require.Empty(t, service.ListManifests())
}

type failingChunkReader struct {
	remaining []byte
}

func (r *failingChunkReader) Read(target []byte) (int, error) {
	if len(r.remaining) == 0 {
		return 0, fmt.Errorf("injected read failure")
	}
	read := copy(target, r.remaining)
	r.remaining = r.remaining[read:]
	if len(r.remaining) == 0 {
		return read, nil
	}
	return read, io.ErrNoProgress
}

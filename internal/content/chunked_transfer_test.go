package content_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"

	"ardents/internal/content"
	data "ardents/internal/content"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
	"ardents/internal/transfer"

	"github.com/stretchr/testify/require"
)

func TestFetchChunkedResumesVerifiedLocalChunks(t *testing.T) {
	dir := t.TempDir()
	store := data.NewInDir(dir)
	require.NoError(t, store.Load())
	history := transfer.NewJournal(storage.PathInDir(dir))
	require.NoError(t, history.Load())
	key := bytes.Repeat([]byte{0x61}, 32)
	stored, err := store.StoreChunkedPayload(context.Background(), data.ChunkedPayloadSpec{
		Owner: externalContentTestOwner(t, 0x33), MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(bytes.Repeat([]byte("payload"), 10000)), key)
	require.NoError(t, err)

	result, err := transfer.FetchChunked(context.Background(), transfer.ExchangeConfig{Data: store, History: history}, stored.Root.ID, transfer.ChunkFetchOptions{Concurrency: 2})
	require.NoError(t, err)
	require.Equal(t, stored.ChunkCount, result.ChunkCount)
	require.Equal(t, 0, result.FetchedCount)
	require.Equal(t, stored.ChunkCount, result.ResumedCount)
	require.Positive(t, result.TotalBytes)

	transfers := history.List()
	require.Len(t, transfers, 1)
	require.Equal(t, "completed", transfers[0].State)
	require.Equal(t, transfers[0].TotalBytes, transfers[0].ProgressBytes)
}

func TestFetchChunkedCancellationIsTerminalAndKeepsChunks(t *testing.T) {
	dir := t.TempDir()
	store := data.NewInDir(dir)
	require.NoError(t, store.Load())
	history := transfer.NewJournal(storage.PathInDir(dir))
	require.NoError(t, history.Load())
	key := bytes.Repeat([]byte{0x62}, 32)
	stored, err := store.StoreChunkedPayload(context.Background(), data.ChunkedPayloadSpec{
		Owner: externalContentTestOwner(t, 0x33), MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(bytes.Repeat([]byte("payload"), content.PlaintextChunkSize)), key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = transfer.FetchChunked(ctx, transfer.ExchangeConfig{Data: store, History: history}, stored.Root.ID, transfer.ChunkFetchOptions{})
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, store.ListBlobs(), stored.ChunkCount)
	transfers := history.List()
	require.Len(t, transfers, 1)
	require.Equal(t, "failed", transfers[0].State)
}

func externalContentTestOwner(t *testing.T, marker byte) identityprincipal.ID {
	t.Helper()
	seed := bytes.Repeat([]byte{marker}, ed25519.SeedSize)
	owner, err := identityprincipal.FromEd25519PublicKey(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return owner
}

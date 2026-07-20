package transfer

import (
	"bytes"
	"context"
	"testing"

	appdata "ardents/internal/data"
	"ardents/internal/data/chunking"

	"github.com/stretchr/testify/require"
)

func TestFetchChunkedResumesVerifiedLocalChunks(t *testing.T) {
	store := appdata.NewInDir(t.TempDir())
	require.NoError(t, store.Load())
	key := bytes.Repeat([]byte{0x61}, 32)
	stored, err := store.StoreChunkedPayload(context.Background(), appdata.ChunkedPayloadSpec{
		Owner: "owner", MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(bytes.Repeat([]byte("payload"), 10000)), key)
	require.NoError(t, err)

	result, err := FetchChunked(context.Background(), ExchangeConfig{Data: store}, stored.Root.ID, ChunkFetchOptions{Concurrency: 2})
	require.NoError(t, err)
	require.Equal(t, stored.ChunkCount, result.ChunkCount)
	require.Equal(t, 0, result.FetchedCount)
	require.Equal(t, stored.ChunkCount, result.ResumedCount)
	require.Positive(t, result.TotalBytes)

	transfers := store.ListTransfers()
	require.Len(t, transfers, 1)
	require.Equal(t, "completed", transfers[0].State)
	require.Equal(t, transfers[0].TotalBytes, transfers[0].ProgressBytes)
}

func TestFetchChunkedCancellationIsTerminalAndKeepsChunks(t *testing.T) {
	store := appdata.NewInDir(t.TempDir())
	require.NoError(t, store.Load())
	key := bytes.Repeat([]byte{0x62}, 32)
	stored, err := store.StoreChunkedPayload(context.Background(), appdata.ChunkedPayloadSpec{
		Owner: "owner", MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(bytes.Repeat([]byte("payload"), chunking.PlaintextChunkSize)), key)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = FetchChunked(ctx, ExchangeConfig{Data: store}, stored.Root.ID, ChunkFetchOptions{})
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, store.ListBlobs(), stored.ChunkCount)
	transfers := store.ListTransfers()
	require.Len(t, transfers, 1)
	require.Equal(t, "failed", transfers[0].State)
}

package transfer

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	appdata "ardents/internal/data"
	"ardents/internal/data/chunking"

	"github.com/stretchr/testify/require"
)

func TestChunkWorkersResumeAfterInterruptedTransfer(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 6)
	transferID := startChunkWorkerTransfer(t, target, plan)
	var calls atomic.Int64
	interrupted := func(_ context.Context, id string) (appdata.Blob, error) {
		if calls.Add(1) > 2 {
			return appdata.Blob{}, fmt.Errorf("injected interruption")
		}
		return copyFixtureChunk(source, target, id)
	}
	_, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target}, transferID, plan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: time.Second}, interrupted)
	require.ErrorContains(t, err, "injected interruption")
	require.Len(t, target.ListBlobs(), 2)

	transferID = startChunkWorkerTransfer(t, target, plan)
	result, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target}, transferID, plan, ChunkFetchOptions{Concurrency: 2, PerChunkTimeout: time.Second}, func(_ context.Context, id string) (appdata.Blob, error) {
		return copyFixtureChunk(source, target, id)
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.Resumed)
	require.Equal(t, 4, result.Fetched)
}

func TestChunkWorkersRejectCorruptChunk(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 1)
	transferID := startChunkWorkerTransfer(t, target, plan)
	_, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target}, transferID, plan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: time.Second}, func(_ context.Context, id string) (appdata.Blob, error) {
		blob, _ := source.GetBlob(id)
		return target.StoreBlob(blob, []byte("corrupt ciphertext"))
	})
	require.ErrorContains(t, err, "mismatch")
	require.Empty(t, target.ListBlobs())
}

func TestChunkWorkersBoundConcurrencyAndSlowPeerTimeout(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 8)
	transferID := startChunkWorkerTransfer(t, target, plan)
	var active atomic.Int64
	var maximum atomic.Int64
	result, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target}, transferID, plan, ChunkFetchOptions{Concurrency: 3, PerChunkTimeout: time.Second}, func(ctx context.Context, id string) (appdata.Blob, error) {
		current := active.Add(1)
		defer active.Add(-1)
		updateMaximum(&maximum, current)
		select {
		case <-ctx.Done():
			return appdata.Blob{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
		return copyFixtureChunk(source, target, id)
	})
	require.NoError(t, err)
	require.Equal(t, 8, result.Fetched)
	require.GreaterOrEqual(t, maximum.Load(), int64(2))
	require.LessOrEqual(t, maximum.Load(), int64(3))

	_, slowTarget, slowPlan := chunkWorkerFixture(t, 1)
	transferID = startChunkWorkerTransfer(t, slowTarget, slowPlan)
	_, err = fetchChunkSet(context.Background(), ExchangeConfig{Data: slowTarget}, transferID, slowPlan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: 20 * time.Millisecond}, func(ctx context.Context, _ string) (appdata.Blob, error) {
		<-ctx.Done()
		return appdata.Blob{}, ctx.Err()
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestChunkWorkersPropagateCancellation(t *testing.T) {
	_, target, plan := chunkWorkerFixture(t, 2)
	transferID := startChunkWorkerTransfer(t, target, plan)
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := fetchChunkSet(ctx, ExchangeConfig{Data: target}, transferID, plan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: time.Second}, func(fetchCtx context.Context, _ string) (appdata.Blob, error) {
			started <- struct{}{}
			<-fetchCtx.Done()
			return appdata.Blob{}, fetchCtx.Err()
		})
		done <- err
	}()
	<-started
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
}

func TestChunkRateLimiterUsesCanonicalChunkBurst(t *testing.T) {
	limiter := chunkRateLimiter(2048)
	require.NotNil(t, limiter)
	require.Equal(t, 2048.0, float64(limiter.Limit()))
	require.Equal(t, chunking.PlaintextChunkSize+32, limiter.Burst())
	require.Nil(t, chunkRateLimiter(0))
}

func TestChunkWorkersAllowTwoBoundedConcurrentTransfers(t *testing.T) {
	sourceOne, target, planOne := chunkWorkerFixture(t, 3)
	sourceTwo, _, planTwo := chunkWorkerFixture(t, 4)
	type outcome struct {
		result chunkWorkerResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	transferOne := startChunkWorkerTransfer(t, target, planOne)
	transferTwo := startChunkWorkerTransfer(t, target, planTwo)
	run := func(source *appdata.Service, transferID string, plan chunking.ResolvedPlan) {
		result, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target}, transferID, plan, ChunkFetchOptions{Concurrency: 2, PerChunkTimeout: time.Second}, func(_ context.Context, id string) (appdata.Blob, error) {
			return copyFixtureChunk(source, target, id)
		})
		outcomes <- outcome{result: result, err: err}
	}
	go run(sourceOne, transferOne, planOne)
	go run(sourceTwo, transferTwo, planTwo)
	first, second := <-outcomes, <-outcomes
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, 7, first.result.Fetched+second.result.Fetched)
	require.Len(t, target.ListBlobs(), 7)
}

func chunkWorkerFixture(t *testing.T, count int) (*appdata.Service, *appdata.Service, chunking.ResolvedPlan) {
	t.Helper()
	source := appdata.NewInDir(t.TempDir())
	target := appdata.NewInDir(t.TempDir())
	require.NoError(t, source.Load())
	require.NoError(t, target.Load())
	key := bytes.Repeat([]byte{0x73}, 32)
	payload := bytes.Repeat([]byte{0x44}, count*chunking.PlaintextChunkSize)
	stored, err := source.StoreChunkedPayload(context.Background(), appdata.ChunkedPayloadSpec{
		Owner: "owner", MediaType: "application/octet-stream", KeyID: "key-1",
	}, bytes.NewReader(payload), key)
	require.NoError(t, err)
	plan, err := chunking.Resolve(stored.Root, nil)
	require.NoError(t, err)
	return source, target, plan
}

func startChunkWorkerTransfer(t *testing.T, target *appdata.Service, plan chunking.ResolvedPlan) string {
	t.Helper()
	transfer, err := target.StartTransfer(appdata.TransferRecord{Kind: "chunked_fetch", State: "pending", TotalBytes: encryptedPlanBytes(plan)})
	require.NoError(t, err)
	return transfer.ID
}

func copyFixtureChunk(source, target *appdata.Service, id string) (appdata.Blob, error) {
	blob, ok := source.GetBlob(id)
	if !ok {
		return appdata.Blob{}, fmt.Errorf("fixture blob missing")
	}
	raw, err := source.GetBlobPayload(id)
	if err != nil {
		return appdata.Blob{}, err
	}
	return target.StoreBlob(blob, raw)
}

func updateMaximum(maximum *atomic.Int64, value int64) {
	for value > maximum.Load() {
		if maximum.CompareAndSwap(maximum.Load(), value) {
			return
		}
	}
}

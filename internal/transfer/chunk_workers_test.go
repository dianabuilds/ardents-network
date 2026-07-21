package transfer

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	chunking "ardents/internal/content"
	model "ardents/internal/content/catalog"
	"ardents/internal/content/payload"

	"github.com/stretchr/testify/require"
)

var chunkFixtureSequence atomic.Int64

func TestChunkWorkersResumeAfterInterruptedTransfer(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 6)
	transferID := startChunkWorkerTransfer(t, target, plan)
	var calls atomic.Int64
	interrupted := func(_ context.Context, id string) (model.Blob, error) {
		if calls.Add(1) > 2 {
			return model.Blob{}, fmt.Errorf("injected interruption")
		}
		return copyFixtureChunk(source, target, id)
	}
	_, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target, History: target}, transferID, plan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: time.Second}, interrupted)
	require.ErrorContains(t, err, "injected interruption")
	require.Equal(t, 2, target.blobCount())

	transferID = startChunkWorkerTransfer(t, target, plan)
	result, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target, History: target}, transferID, plan, ChunkFetchOptions{Concurrency: 2, PerChunkTimeout: time.Second}, func(_ context.Context, id string) (model.Blob, error) {
		return copyFixtureChunk(source, target, id)
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.Resumed)
	require.Equal(t, 4, result.Fetched)
}

func TestChunkWorkersRejectCorruptChunk(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 1)
	transferID := startChunkWorkerTransfer(t, target, plan)
	_, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target, History: target}, transferID, plan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: time.Second}, func(_ context.Context, id string) (model.Blob, error) {
		blob, _ := source.GetBlob(id)
		return target.StoreBlob(blob, []byte("corrupt ciphertext"))
	})
	require.ErrorContains(t, err, "mismatch")
	require.Zero(t, target.blobCount())
}

func TestChunkWorkersBoundConcurrencyAndSlowPeerTimeout(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 8)
	transferID := startChunkWorkerTransfer(t, target, plan)
	var active atomic.Int64
	var maximum atomic.Int64
	result, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target, History: target}, transferID, plan, ChunkFetchOptions{Concurrency: 3, PerChunkTimeout: time.Second}, func(ctx context.Context, id string) (model.Blob, error) {
		current := active.Add(1)
		defer active.Add(-1)
		updateMaximum(&maximum, current)
		select {
		case <-ctx.Done():
			return model.Blob{}, ctx.Err()
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
	_, err = fetchChunkSet(context.Background(), ExchangeConfig{Data: slowTarget, History: slowTarget}, transferID, slowPlan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: 20 * time.Millisecond}, func(ctx context.Context, _ string) (model.Blob, error) {
		<-ctx.Done()
		return model.Blob{}, ctx.Err()
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
		_, err := fetchChunkSet(ctx, ExchangeConfig{Data: target, History: target}, transferID, plan, ChunkFetchOptions{Concurrency: 1, PerChunkTimeout: time.Second}, func(fetchCtx context.Context, _ string) (model.Blob, error) {
			started <- struct{}{}
			<-fetchCtx.Done()
			return model.Blob{}, fetchCtx.Err()
		})
		done <- err
	}()
	<-started
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
}

func TestChunkWorkersSurfaceProgressPersistenceFailure(t *testing.T) {
	source, target, plan := chunkWorkerFixture(t, 1)
	transferID := startChunkWorkerTransfer(t, target, plan)
	data := failingProgressData{DataExchange: target, err: fmt.Errorf("disk unavailable")}

	_, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: data, History: data}, transferID, plan, ChunkFetchOptions{
		Concurrency: 1, PerChunkTimeout: time.Second,
	}, func(_ context.Context, id string) (model.Blob, error) {
		return copyFixtureChunk(source, target, id)
	})

	require.ErrorContains(t, err, "record transfer progress")
	require.ErrorContains(t, err, "disk unavailable")
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
	run := func(source *chunkTestData, transferID string, plan chunking.ResolvedPlan) {
		result, err := fetchChunkSet(context.Background(), ExchangeConfig{Data: target, History: target}, transferID, plan, ChunkFetchOptions{Concurrency: 2, PerChunkTimeout: time.Second}, func(_ context.Context, id string) (model.Blob, error) {
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
	require.Equal(t, 7, target.blobCount())
}

func chunkWorkerFixture(t *testing.T, count int) (*chunkTestData, *chunkTestData, chunking.ResolvedPlan) {
	t.Helper()
	source, target := newChunkTestData(), newChunkTestData()
	ids := make([]string, 0, count)
	fixtureID := byte(chunkFixtureSequence.Add(1))
	for index := range count {
		raw := bytes.Repeat([]byte{fixtureID, byte(index + 1)}, 32)
		blob, err := source.StoreBlob(model.Blob{Encrypted: true, Retention: "fetched"}, raw)
		require.NoError(t, err)
		ids = append(ids, blob.ID)
	}
	return source, target, chunking.ResolvedPlan{
		ChunkIDs: ids, TotalPlaintextBytes: int64(count * chunking.PlaintextChunkSize),
	}
}

func startChunkWorkerTransfer(t *testing.T, target *chunkTestData, plan chunking.ResolvedPlan) string {
	t.Helper()
	transfer, err := target.Start(Record{Kind: "chunked_fetch", State: "pending", TotalBytes: encryptedPlanBytes(plan)})
	require.NoError(t, err)
	return transfer.ID
}

func copyFixtureChunk(source, target *chunkTestData, id string) (model.Blob, error) {
	blob, ok := source.GetBlob(id)
	if !ok {
		return model.Blob{}, fmt.Errorf("fixture blob missing")
	}
	raw, err := source.GetBlobPayload(id)
	if err != nil {
		return model.Blob{}, err
	}
	return target.StoreBlob(blob, raw)
}

type chunkTestData struct {
	DataExchange
	History
	mu        sync.Mutex
	blobs     map[string]model.Blob
	payloads  map[string][]byte
	transfers map[string]Record
	nextID    int
}

func newChunkTestData() *chunkTestData {
	return &chunkTestData{
		blobs: map[string]model.Blob{}, payloads: map[string][]byte{},
		transfers: map[string]Record{},
	}
}

func (d *chunkTestData) GetBlob(id string) (model.Blob, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	blob, ok := d.blobs[id]
	return blob, ok
}

func (d *chunkTestData) GetBlobPayload(id string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	raw, ok := d.payloads[id]
	if !ok {
		return nil, fmt.Errorf("fixture payload missing")
	}
	return append([]byte(nil), raw...), nil
}

func (d *chunkTestData) StoreBlob(blob model.Blob, raw []byte) (model.Blob, error) {
	hash, blobCID, err := payload.DeriveIdentity(raw)
	if err != nil {
		return model.Blob{}, err
	}
	if err := payload.ApplyDerivedIdentity(&blob, hash, blobCID); err != nil {
		return model.Blob{}, err
	}
	blob.Size = int64(len(raw))
	d.mu.Lock()
	defer d.mu.Unlock()
	d.blobs[blob.ID], d.payloads[blob.ID] = blob, append([]byte(nil), raw...)
	return blob, nil
}

func (d *chunkTestData) Start(record Record) (Record, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	if record.ID == "" {
		record.ID = fmt.Sprintf("transfer-%d", d.nextID)
	}
	d.transfers[record.ID] = record
	return record, nil
}

func (d *chunkTestData) Progress(id string, progress, total int64, reason string) (Record, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	record := d.transfers[id]
	record.ProgressBytes, record.TotalBytes, record.Reason = progress, total, reason
	d.transfers[id] = record
	return record, nil
}

func (d *chunkTestData) blobCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.blobs)
}

func updateMaximum(maximum *atomic.Int64, value int64) {
	for value > maximum.Load() {
		if maximum.CompareAndSwap(maximum.Load(), value) {
			return
		}
	}
}

type failingProgressData struct {
	DataExchange
	History
	err error
}

func (d failingProgressData) Progress(string, int64, int64, string) (Record, error) {
	return Record{}, d.err
}

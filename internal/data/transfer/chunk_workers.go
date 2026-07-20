package transfer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	appdata "ardents/internal/data"
	"ardents/internal/data/chunking"
	"ardents/internal/data/payload"

	"golang.org/x/time/rate"
)

type chunkWorkerResult struct {
	Fetched    int
	Resumed    int
	TotalBytes int64
}

type chunkProgress struct {
	fetched atomic.Int64
	resumed atomic.Int64
	bytes   atomic.Int64
}

type chunkFetchFunc func(context.Context, string) (appdata.Blob, error)

func fetchChunkSet(ctx context.Context, cfg ExchangeConfig, transferID string, plan chunking.ResolvedPlan, options ChunkFetchOptions, fetch chunkFetchFunc) (chunkWorkerResult, error) {
	parentCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan string)
	errorsFound := make(chan error, 1)
	progress := &chunkProgress{}
	limiter := chunkRateLimiter(options.BytesPerSecond)
	var workers sync.WaitGroup
	for range options.Concurrency {
		workers.Add(1)
		go chunkWorker(ctx, &workers, cfg, transferID, plan, options, limiter, jobs, progress, errorsFound, cancel, fetch)
	}
	go sendChunkJobs(ctx, plan.ChunkIDs, jobs)
	workers.Wait()
	select {
	case err := <-errorsFound:
		return chunkWorkerResult{}, err
	default:
	}
	if err := parentCtx.Err(); err != nil {
		return chunkWorkerResult{}, err
	}
	return chunkWorkerResult{Fetched: int(progress.fetched.Load()), Resumed: int(progress.resumed.Load()), TotalBytes: progress.bytes.Load()}, nil
}

func chunkWorker(
	ctx context.Context, workers *sync.WaitGroup, cfg ExchangeConfig, transferID string,
	plan chunking.ResolvedPlan, options ChunkFetchOptions, limiter *rate.Limiter,
	jobs <-chan string, progress *chunkProgress, errorsFound chan<- error, cancel context.CancelFunc,
	fetch chunkFetchFunc,
) {
	defer workers.Done()
	for id := range jobs {
		bytes, resumed, err := fetchOneChunk(ctx, cfg, id, options, limiter, fetch)
		if err != nil {
			select {
			case errorsFound <- err:
			default:
			}
			cancel()
			return
		}
		if resumed {
			progress.resumed.Add(1)
		} else {
			progress.fetched.Add(1)
		}
		current := progress.bytes.Add(bytes)
		completed := progress.fetched.Load() + progress.resumed.Load()
		if completed%16 == 0 || completed == int64(len(plan.ChunkIDs)) {
			if err := updateTransferProgress(cfg.Data, transferID, current, encryptedPlanBytes(plan), "verified manifest chunks"); err != nil {
				select {
				case errorsFound <- err:
				default:
				}
				cancel()
				return
			}
		}
	}
}

func fetchOneChunk(ctx context.Context, cfg ExchangeConfig, id string, options ChunkFetchOptions, limiter *rate.Limiter, fetch chunkFetchFunc) (int64, bool, error) {
	if size, ok := validLocalChunk(cfg.Data, id); ok {
		return size, true, nil
	}
	if limiter != nil {
		if err := limiter.WaitN(ctx, chunking.PlaintextChunkSize+32); err != nil {
			return 0, false, err
		}
	}
	chunkCtx, cancel := context.WithTimeout(ctx, options.PerChunkTimeout)
	defer cancel()
	if fetch == nil {
		return 0, false, fmt.Errorf("chunk fetch implementation is unavailable")
	}
	if _, err := fetch(chunkCtx, id); err != nil {
		return 0, false, err
	}
	size, ok := validLocalChunk(cfg.Data, id)
	if !ok {
		return 0, false, fmt.Errorf("fetched chunk failed content verification")
	}
	return size, false, nil
}

func validLocalChunk(data DataExchange, id string) (int64, bool) {
	blob, ok := data.GetBlob(id)
	if !ok || !blob.Encrypted || blob.Retention == "staging" {
		return 0, false
	}
	raw, err := data.GetBlobPayload(id)
	if err != nil {
		return 0, false
	}
	hash, cid, err := payload.DeriveIdentity(raw)
	if err != nil || cid != id || blob.CID != cid || blob.Hash != hash {
		return 0, false
	}
	return int64(len(raw)), true
}

func sendChunkJobs(ctx context.Context, ids []string, jobs chan<- string) {
	defer close(jobs)
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return
		case jobs <- id:
		}
	}
}

func chunkRateLimiter(bytesPerSecond int64) *rate.Limiter {
	if bytesPerSecond <= 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(bytesPerSecond), chunking.PlaintextChunkSize+32)
}

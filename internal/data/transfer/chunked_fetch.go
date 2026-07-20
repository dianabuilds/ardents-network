package transfer

import (
	"context"
	"fmt"
	"time"

	appdata "ardents/internal/data"
	"ardents/internal/data/chunking"
)

type ChunkFetchOptions struct {
	Concurrency     int
	PerChunkTimeout time.Duration
	BytesPerSecond  int64
}

type ChunkFetchResult struct {
	Root         appdata.Manifest
	ChunkCount   int
	FetchedCount int
	ResumedCount int
	TotalBytes   int64
}

func FetchChunked(ctx context.Context, cfg ExchangeConfig, rootID string, options ChunkFetchOptions) (ChunkFetchResult, error) {
	if cfg.Data == nil || rootID == "" {
		return ChunkFetchResult{}, fmt.Errorf("chunked fetch dependencies are unavailable")
	}
	plan, err := loadChunkPlan(ctx, cfg, rootID)
	if err != nil {
		return ChunkFetchResult{}, err
	}
	transfer, err := cfg.Data.StartTransfer(appdata.TransferRecord{
		Kind: "chunked_fetch", ResourceID: rootID, Direction: "inbound", State: "pending",
		TotalBytes: encryptedPlanBytes(plan), Reason: "fetching bounded manifest chunks",
	})
	if err != nil {
		return ChunkFetchResult{}, err
	}
	workerResult, err := fetchChunkSet(ctx, cfg, transfer.ID, plan, normalizeChunkOptions(options), func(fetchCtx context.Context, id string) (appdata.Blob, error) {
		return FetchBlob(fetchCtx, cfg, id)
	})
	if err != nil {
		recordChunkFetchFailure(cfg, transfer.ID, rootID, err)
		return ChunkFetchResult{}, err
	}
	if err := publishResolvedPlan(cfg.Data, plan); err != nil {
		recordChunkFetchFailure(cfg, transfer.ID, rootID, err)
		return ChunkFetchResult{}, err
	}
	_, _ = cfg.Data.CompleteTransfer(transfer.ID, "", workerResult.TotalBytes, "chunked manifest fetch completed")
	if cfg.Publish != nil {
		cfg.Publish("data.chunked_fetched", map[string]any{"id": rootID, "chunks": len(plan.ChunkIDs), "bytes": workerResult.TotalBytes})
	}
	if cfg.Diagnostics != nil {
		cfg.Diagnostics.RecordEvent("data", "chunked_fetched", rootID, "chunked manifest fetched", "", map[string]any{"chunks": len(plan.ChunkIDs), "bytes": workerResult.TotalBytes})
	}
	return ChunkFetchResult{
		Root: plan.Root, ChunkCount: len(plan.ChunkIDs), FetchedCount: workerResult.Fetched,
		ResumedCount: workerResult.Resumed, TotalBytes: workerResult.TotalBytes,
	}, nil
}

func recordChunkFetchFailure(cfg ExchangeConfig, transferID, rootID string, err error) {
	_, _ = cfg.Data.FailTransfer(transferID, "", err.Error())
	if cfg.Diagnostics != nil {
		cfg.Diagnostics.RecordEvent("data", "chunked_fetch_failed", rootID, "chunked manifest fetch failed", "data.chunked.fetch_failed", nil)
	}
}

func loadChunkPlan(ctx context.Context, cfg ExchangeConfig, rootID string) (chunking.ResolvedPlan, error) {
	root, ok := validLocalManifest(cfg.Data, rootID)
	if !ok {
		var err error
		root, err = FetchManifest(ctx, cfg, rootID)
		if err != nil {
			return chunking.ResolvedPlan{}, err
		}
	}
	leaves := make(map[string]appdata.Manifest)
	if root.Kind == "chunk-root" {
		for _, ref := range root.Refs {
			leaf, found := validLocalManifest(cfg.Data, ref.ID)
			if !found {
				var err error
				leaf, err = FetchManifest(ctx, cfg, ref.ID)
				if err != nil {
					return chunking.ResolvedPlan{}, err
				}
			}
			leaves[ref.ID] = leaf
		}
	}
	return chunking.Resolve(root, func(id string) (appdata.Manifest, bool) {
		manifest, found := leaves[id]
		return manifest, found
	})
}

func validLocalManifest(data DataExchange, id string) (appdata.Manifest, bool) {
	manifest, ok := data.GetManifest(id)
	if !ok || chunking.ValidateManifest(manifest) != nil {
		return appdata.Manifest{}, false
	}
	return manifest, true
}

func publishResolvedPlan(data DataExchange, plan chunking.ResolvedPlan) error {
	for _, leaf := range plan.Leaves {
		if _, err := data.PublishManifest(leaf); err != nil {
			return err
		}
	}
	if plan.Root.Kind == "chunk-root" {
		_, err := data.PublishManifest(plan.Root)
		return err
	}
	return nil
}

func normalizeChunkOptions(options ChunkFetchOptions) ChunkFetchOptions {
	if options.Concurrency <= 0 {
		options.Concurrency = 4
	}
	if options.Concurrency > 8 {
		options.Concurrency = 8
	}
	if options.PerChunkTimeout <= 0 {
		options.PerChunkTimeout = 15 * time.Second
	}
	return options
}

func encryptedPlanBytes(plan chunking.ResolvedPlan) int64 {
	return plan.TotalPlaintextBytes + int64(len(plan.ChunkIDs))*16
}

package transfer

import (
	"context"
	"fmt"
	"time"

	chunking "ardents/internal/content"
	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
)

type ChunkFetchOptions struct {
	Owner           principal.ID
	Concurrency     int
	PerChunkTimeout time.Duration
	BytesPerSecond  int64
}

type ChunkFetchResult struct {
	Root         model.Manifest
	ChunkCount   int
	FetchedCount int
	ResumedCount int
	TotalBytes   int64
}

func FetchChunked(ctx context.Context, cfg ExchangeConfig, rootID string, options ChunkFetchOptions) (ChunkFetchResult, error) {
	if cfg.Data == nil || cfg.History == nil || rootID == "" || options.Owner.String() == "" {
		return ChunkFetchResult{}, fmt.Errorf("chunked fetch dependencies are unavailable")
	}
	plan, err := loadChunkPlan(ctx, cfg, options.Owner, rootID)
	if err != nil {
		return ChunkFetchResult{}, err
	}
	transfer, err := cfg.History.Start(Record{
		Kind: "chunked_fetch", ResourceOwner: options.Owner.String(), ResourceID: rootID, Direction: "inbound", State: "pending",
		TotalBytes: encryptedPlanBytes(plan), Reason: "fetching bounded manifest chunks",
	})
	if err != nil {
		return ChunkFetchResult{}, err
	}
	workerResult, err := fetchChunkSet(ctx, cfg, transfer.ID, plan, normalizeChunkOptions(options), func(fetchCtx context.Context, id string) (model.Blob, error) {
		return FetchBlob(fetchCtx, cfg, id)
	})
	if err != nil {
		return ChunkFetchResult{}, recordChunkFetchFailure(cfg, transfer.ID, rootID, err)
	}
	if err := publishResolvedPlan(cfg.Data, plan); err != nil {
		return ChunkFetchResult{}, recordChunkFetchFailure(cfg, transfer.ID, rootID, err)
	}
	if err := completeTransfer(cfg.History, transfer.ID, "", workerResult.TotalBytes, "chunked manifest fetch completed"); err != nil {
		return ChunkFetchResult{}, recordChunkFetchFailure(cfg, transfer.ID, rootID, err)
	}
	if cfg.Publish != nil {
		cfg.Publish("data.chunked_fetched", map[string]any{"id": rootID, "chunks": len(plan.ChunkIDs), "bytes": workerResult.TotalBytes})
	}
	if cfg.RecordEvent != nil {
		cfg.RecordEvent("data", "chunked_fetched", rootID, "chunked manifest fetched", "", map[string]any{"chunks": len(plan.ChunkIDs), "bytes": workerResult.TotalBytes})
	}
	return ChunkFetchResult{
		Root: plan.Root, ChunkCount: len(plan.ChunkIDs), FetchedCount: workerResult.Fetched,
		ResumedCount: workerResult.Resumed, TotalBytes: workerResult.TotalBytes,
	}, nil
}

func recordChunkFetchFailure(cfg ExchangeConfig, transferID, rootID string, cause error) error {
	err := failTransfer(cfg.History, transferID, "", cause)
	if cfg.RecordEvent != nil {
		cfg.RecordEvent("data", "chunked_fetch_failed", rootID, "chunked manifest fetch failed", "data.chunked.fetch_failed", nil)
	}
	return err
}

func loadChunkPlan(ctx context.Context, cfg ExchangeConfig, owner principal.ID, rootID string) (chunking.ResolvedPlan, error) {
	root, ok := validLocalManifest(cfg.Data, owner, rootID)
	if !ok {
		var err error
		root, err = FetchManifest(ctx, cfg, owner, rootID)
		if err != nil {
			return chunking.ResolvedPlan{}, err
		}
	}
	leaves := make(map[string]model.Manifest)
	if root.Kind == "chunk-root" {
		for _, ref := range root.Refs {
			leaf, found := validLocalManifest(cfg.Data, root.Owner, ref.ID)
			if !found {
				var err error
				leaf, err = FetchManifest(ctx, cfg, root.Owner, ref.ID)
				if err != nil {
					return chunking.ResolvedPlan{}, err
				}
			}
			leaves[ref.ID] = leaf
		}
	}
	return chunking.Resolve(root, func(id string) (model.Manifest, bool) {
		manifest, found := leaves[id]
		return manifest, found
	})
}

func validLocalManifest(data DataExchange, owner principal.ID, id string) (model.Manifest, bool) {
	manifest, ok := data.ReadTransferManifest(owner, id)
	if !ok || chunking.ValidateManifest(manifest) != nil {
		return model.Manifest{}, false
	}
	return manifest, true
}

func publishResolvedPlan(data DataExchange, plan chunking.ResolvedPlan) error {
	for _, leaf := range plan.Leaves {
		if _, err := data.WriteTransferManifest(leaf); err != nil {
			return err
		}
	}
	if plan.Root.Kind == "chunk-root" {
		_, err := data.WriteTransferManifest(plan.Root)
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

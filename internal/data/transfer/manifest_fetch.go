package transfer

import (
	"context"
	"errors"
	"fmt"
	"time"

	appdata "ardents/internal/data"
)

func FetchManifest(ctx context.Context, cfg ExchangeConfig, manifestID string) (appdata.Manifest, error) {
	if cfg.Identity == nil || cfg.Private == nil || cfg.Data == nil || cfg.Discovery == nil || cfg.Trust == nil {
		return appdata.Manifest{}, fmt.Errorf("manifest fetch dependencies are unavailable")
	}
	requestID := fmt.Sprintf("manifest-fetch-%d", time.Now().UTC().UnixNano())
	requester := cfg.Identity.NodeSummary().Principal
	started, err := cfg.Data.StartTransfer(appdata.TransferRecord{
		ID: requestID, Kind: "manifest_fetch", ResourceID: manifestID, Direction: "inbound",
		State: "pending", Reason: "waiting for remote manifest response",
	})
	if err != nil {
		return appdata.Manifest{}, err
	}
	responses, unregister, err := cfg.Private.RegisterResponse(requestID)
	if err != nil {
		return appdata.Manifest{}, failTransfer(cfg.Data, started.ID, "", err)
	}
	defer unregister()
	if err := publishDataFetchRequest(ctx, cfg, requestID, manifestID, requester, "manifest"); err != nil {
		return appdata.Manifest{}, failTransfer(cfg.Data, started.ID, "", err)
	}
	return awaitManifestResponse(ctx, cfg, started.ID, manifestID, requester, requestID, responses)
}

func awaitManifestResponse(ctx context.Context, cfg ExchangeConfig, transferID, manifestID, requester, requestID string, responses <-chan []byte) (appdata.Manifest, error) {
	var candidateErr error
	var candidatePeer string
	for {
		select {
		case <-ctx.Done():
			return failManifestTransfer(cfg, transferID, candidatePeer, candidateErr, ctx.Err())
		case payload, ok := <-responses:
			if !ok {
				return failManifestTransfer(cfg, transferID, candidatePeer, candidateErr, fmt.Errorf("manifest response stream closed"))
			}
			manifest, peer, err := acceptManifestResponse(cfg, manifestID, requester, requestID, payload)
			if err != nil {
				var terminal blobFetchTerminalError
				if errors.As(err, &terminal) {
					return appdata.Manifest{}, failTransfer(cfg.Data, transferID, peer, terminal.err)
				}
				var candidate blobFetchCandidateError
				if errors.As(err, &candidate) {
					candidateErr, candidatePeer = candidate.err, peer
				}
				continue
			}
			if err := completeTransfer(cfg.Data, transferID, peer, 0, "manifest fetched from trusted peer"); err != nil {
				return appdata.Manifest{}, err
			}
			return manifest, nil
		}
	}
}

func failManifestTransfer(cfg ExchangeConfig, transferID, peer string, candidateErr, fallback error) (appdata.Manifest, error) {
	if candidateErr != nil {
		fallback = candidateErr
	}
	return appdata.Manifest{}, failTransfer(cfg.Data, transferID, peer, fallback)
}

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
		_, _ = cfg.Data.FailTransfer(started.ID, "", err.Error())
		return appdata.Manifest{}, err
	}
	defer unregister()
	if err := publishDataFetchRequest(ctx, cfg, requestID, manifestID, requester, "manifest"); err != nil {
		_, _ = cfg.Data.FailTransfer(started.ID, "", err.Error())
		return appdata.Manifest{}, err
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
					_, _ = cfg.Data.FailTransfer(transferID, peer, terminal.err.Error())
					return appdata.Manifest{}, terminal.err
				}
				var candidate blobFetchCandidateError
				if errors.As(err, &candidate) {
					candidateErr, candidatePeer = candidate.err, peer
				}
				continue
			}
			_, _ = cfg.Data.CompleteTransfer(transferID, peer, 0, "manifest fetched from trusted peer")
			return manifest, nil
		}
	}
}

func failManifestTransfer(cfg ExchangeConfig, transferID, peer string, candidateErr, fallback error) (appdata.Manifest, error) {
	if candidateErr != nil {
		fallback = candidateErr
	}
	_, _ = cfg.Data.FailTransfer(transferID, peer, fallback.Error())
	return appdata.Manifest{}, fallback
}

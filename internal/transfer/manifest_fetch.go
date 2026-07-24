package transfer

import (
	"context"
	"errors"
	"fmt"

	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
)

func FetchManifest(ctx context.Context, cfg ExchangeConfig, owner principal.ID, manifestID string) (model.Manifest, error) {
	if owner.String() == "" {
		return model.Manifest{}, fmt.Errorf("manifest owner is required")
	}
	if cfg.Identity == nil || cfg.Private == nil || cfg.Data == nil || cfg.History == nil || cfg.Discovery == nil || cfg.Trust == nil {
		return model.Manifest{}, fmt.Errorf("manifest fetch dependencies are unavailable")
	}
	requestID, err := requestIdentity("manifest-fetch")
	if err != nil {
		return model.Manifest{}, err
	}
	requester := cfg.Identity().Principal
	started, err := cfg.History.Start(Record{
		ID: requestID, Kind: "manifest_fetch", ResourceOwner: owner.String(), ResourceID: manifestID, Direction: "inbound",
		State: "pending", Reason: "waiting for remote manifest response",
	})
	if err != nil {
		return model.Manifest{}, err
	}
	responses, unregister, err := cfg.Private.RegisterResponse(requestID)
	if err != nil {
		return model.Manifest{}, failTransfer(cfg.History, started.ID, "", err)
	}
	defer unregister()
	if err := publishOwnedDataFetchRequest(ctx, cfg, requestID, manifestID, requester, "manifest", owner.String()); err != nil {
		return model.Manifest{}, failTransfer(cfg.History, started.ID, "", err)
	}
	return awaitManifestResponse(ctx, cfg, started.ID, owner, manifestID, requester, requestID, responses)
}

func awaitManifestResponse(ctx context.Context, cfg ExchangeConfig, transferID string, owner principal.ID, manifestID, requester, requestID string, responses <-chan []byte) (model.Manifest, error) {
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
			manifest, peer, err := acceptManifestResponse(cfg, owner, manifestID, requester, requestID, payload)
			if err != nil {
				if terminal, ok := errors.AsType[blobFetchTerminalError](err); ok {
					return model.Manifest{}, failTransfer(cfg.History, transferID, peer, terminal.err)
				}
				if candidate, ok := errors.AsType[blobFetchCandidateError](err); ok {
					candidateErr, candidatePeer = candidate.err, peer
				}
				continue
			}
			if err := completeTransfer(cfg.History, transferID, peer, 0, "manifest fetched from trusted peer"); err != nil {
				return model.Manifest{}, err
			}
			return manifest, nil
		}
	}
}

func failManifestTransfer(cfg ExchangeConfig, transferID, peer string, candidateErr, fallback error) (model.Manifest, error) {
	if candidateErr != nil {
		fallback = candidateErr
	}
	return model.Manifest{}, failTransfer(cfg.History, transferID, peer, fallback)
}

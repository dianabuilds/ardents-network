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
	ctx, cancel := boundedFetchContext(ctx, cfg.FetchTimeout)
	defer cancel()
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
	candidates := trustedFetchCandidates(cfg, requester)
	if len(candidates) == 0 {
		return model.Manifest{}, failTransfer(cfg.History, started.ID, "", fmt.Errorf("no trusted fetch candidates available"))
	}
	if err := publishOwnedDataFetchRequest(ctx, cfg, requestID, manifestID, requester, "manifest", owner.String()); err != nil {
		return model.Manifest{}, failTransfer(cfg.History, started.ID, "", err)
	}
	return awaitManifestResponse(ctx, cfg, started.ID, owner, manifestID, requester, requestID, responses, candidates)
}

func awaitManifestResponse(ctx context.Context, cfg ExchangeConfig, transferID string, owner principal.ID, manifestID, requester, requestID string, responses <-chan []byte, candidateSets ...fetchCandidateSet) (model.Manifest, error) {
	var tracker *fetchCandidateTracker
	if len(candidateSets) > 0 {
		tracker = newFetchCandidateTracker(candidateSets[0])
		if len(tracker.candidates) == 0 {
			return model.Manifest{}, failTransfer(cfg.History, transferID, "", fmt.Errorf("no trusted fetch candidates available"))
		}
	}
	var candidateErr error
	var candidateNodeID string
	for {
		select {
		case <-ctx.Done():
			if tracker != nil {
				return model.Manifest{}, failTransfer(cfg.History, transferID, candidateNodeID, tracker.incomplete(ctx.Err()))
			}
			return failManifestTransfer(cfg, transferID, candidateNodeID, candidateErr, ctx.Err())
		case payload, ok := <-responses:
			if !ok {
				if tracker != nil {
					return model.Manifest{}, failTransfer(cfg.History, transferID, candidateNodeID, tracker.incomplete(fmt.Errorf("manifest response stream closed")))
				}
				return failManifestTransfer(cfg, transferID, candidateNodeID, candidateErr, fmt.Errorf("manifest response stream closed"))
			}
			manifest, nodeID, err := acceptManifestResponse(cfg, owner, manifestID, requester, requestID, payload, tracker)
			if err != nil {
				if candidate, ok := errors.AsType[blobFetchCandidateError](err); ok {
					candidateErr, candidateNodeID = candidate.err, nodeID
					if exhausted := tracker.fail(nodeID, candidate.err); exhausted != nil {
						return model.Manifest{}, failTransfer(cfg.History, transferID, nodeID, exhausted)
					}
				}
				continue
			}
			if err := completeTransfer(cfg.History, transferID, nodeID, 0, "manifest fetched from trusted node"); err != nil {
				return model.Manifest{}, err
			}
			return manifest, nil
		}
	}
}

func failManifestTransfer(cfg ExchangeConfig, transferID, nodeID string, candidateErr, fallback error) (model.Manifest, error) {
	if candidateErr != nil {
		fallback = candidateErr
	}
	return model.Manifest{}, failTransfer(cfg.History, transferID, nodeID, fallback)
}

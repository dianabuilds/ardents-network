package transfer

import (
	"context"
	"errors"
	"fmt"

	model "ardents/internal/content/catalog"
)

type blobFetchCandidateError struct {
	err error
}

func (e blobFetchCandidateError) Error() string {
	return e.err.Error()
}

func awaitBlobFetchResponse(ctx context.Context, cfg ExchangeConfig, transferID, blobID, requester, requestID string, responses <-chan []byte, candidateSets ...fetchCandidateSet) (model.Blob, error) {
	var tracker *fetchCandidateTracker
	if len(candidateSets) > 0 {
		tracker = newFetchCandidateTracker(candidateSets[0])
		if len(tracker.candidates) == 0 {
			return model.Blob{}, failTransfer(cfg.History, transferID, "", fmt.Errorf("no trusted fetch candidates available"))
		}
	}
	var candidateErr error
	var candidateNodeID string
	for {
		select {
		case <-ctx.Done():
			if tracker != nil {
				return model.Blob{}, failTransfer(cfg.History, transferID, candidateNodeID, tracker.incomplete(ctx.Err()))
			}
			if candidateErr != nil {
				return model.Blob{}, failTransfer(cfg.History, transferID, candidateNodeID, candidateErr)
			}
			err := ctx.Err()
			return model.Blob{}, failTransfer(cfg.History, transferID, candidateNodeID, err)
		case payload, ok := <-responses:
			if !ok {
				if tracker != nil {
					return model.Blob{}, failTransfer(cfg.History, transferID, candidateNodeID, tracker.incomplete(fmt.Errorf("blob response stream closed")))
				}
				return failAwaitedTransfer(cfg, transferID, candidateNodeID, candidateErr, fmt.Errorf("blob response stream closed"))
			}
			blob, nodeID, totalBytes, err := acceptBlobResponse(cfg, blobID, requester, requestID, payload, tracker)
			if err != nil {
				if rememberedErr, ok := errors.AsType[blobFetchCandidateError](err); ok {
					candidateErr = rememberedErr.err
					candidateNodeID = nodeID
					if exhausted := tracker.fail(nodeID, rememberedErr.err); exhausted != nil {
						return model.Blob{}, failTransfer(cfg.History, transferID, nodeID, exhausted)
					}
				}
				continue
			}
			if err := completeTransfer(cfg.History, transferID, nodeID, totalBytes, "blob fetched from trusted node"); err != nil {
				return model.Blob{}, err
			}
			return blob, nil
		}
	}
}

func failAwaitedTransfer(cfg ExchangeConfig, transferID, candidateNodeID string, candidateErr, fallback error) (model.Blob, error) {
	if candidateErr != nil {
		return model.Blob{}, failTransfer(cfg.History, transferID, candidateNodeID, candidateErr)
	}
	return model.Blob{}, failTransfer(cfg.History, transferID, candidateNodeID, fallback)
}

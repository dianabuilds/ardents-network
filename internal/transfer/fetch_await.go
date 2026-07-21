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

func awaitBlobFetchResponse(ctx context.Context, cfg ExchangeConfig, transferID, blobID, requester, requestID string, responses <-chan []byte) (model.Blob, error) {
	var candidateErr error
	var candidatePeer string
	for {
		select {
		case <-ctx.Done():
			if candidateErr != nil {
				return model.Blob{}, failTransfer(cfg.History, transferID, candidatePeer, candidateErr)
			}
			err := ctx.Err()
			return model.Blob{}, failTransfer(cfg.History, transferID, candidatePeer, err)
		case payload, ok := <-responses:
			if !ok {
				return failAwaitedTransfer(cfg, transferID, candidatePeer, candidateErr, fmt.Errorf("blob response stream closed"))
			}
			blob, peer, totalBytes, err := acceptBlobResponse(cfg, blobID, requester, requestID, payload)
			if err != nil {
				if terminalErr, ok := errors.AsType[blobFetchTerminalError](err); ok {
					return model.Blob{}, failTransfer(cfg.History, transferID, peer, terminalErr.err)
				}
				if rememberedErr, ok := errors.AsType[blobFetchCandidateError](err); ok {
					candidateErr = rememberedErr.err
					candidatePeer = peer
				}
				continue
			}
			if err := completeTransfer(cfg.History, transferID, peer, totalBytes, "blob fetched from trusted peer"); err != nil {
				return model.Blob{}, err
			}
			return blob, nil
		}
	}
}

func failAwaitedTransfer(cfg ExchangeConfig, transferID, candidatePeer string, candidateErr, fallback error) (model.Blob, error) {
	if candidateErr != nil {
		return model.Blob{}, failTransfer(cfg.History, transferID, candidatePeer, candidateErr)
	}
	return model.Blob{}, failTransfer(cfg.History, transferID, candidatePeer, fallback)
}

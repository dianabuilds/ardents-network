package transfer

import (
	"context"
	"errors"
	"fmt"

	appdata "ardents/internal/data"
)

type blobFetchCandidateError struct {
	err error
}

func (e blobFetchCandidateError) Error() string {
	return e.err.Error()
}

func awaitBlobFetchResponse(ctx context.Context, cfg ExchangeConfig, transferID, blobID, requester, requestID string, responses <-chan []byte) (appdata.Blob, error) {
	var candidateErr error
	var candidatePeer string
	for {
		select {
		case <-ctx.Done():
			if candidateErr != nil {
				return appdata.Blob{}, failTransfer(cfg.Data, transferID, candidatePeer, candidateErr)
			}
			err := ctx.Err()
			return appdata.Blob{}, failTransfer(cfg.Data, transferID, candidatePeer, err)
		case payload, ok := <-responses:
			if !ok {
				return failAwaitedTransfer(cfg, transferID, candidatePeer, candidateErr, fmt.Errorf("blob response stream closed"))
			}
			blob, peer, totalBytes, err := acceptBlobResponse(cfg, blobID, requester, requestID, payload)
			if err != nil {
				var terminalErr blobFetchTerminalError
				if errors.As(err, &terminalErr) {
					return appdata.Blob{}, failTransfer(cfg.Data, transferID, peer, terminalErr.err)
				}
				var rememberedErr blobFetchCandidateError
				if errors.As(err, &rememberedErr) {
					candidateErr = rememberedErr.err
					candidatePeer = peer
				}
				continue
			}
			if err := completeTransfer(cfg.Data, transferID, peer, totalBytes, "blob fetched from trusted peer"); err != nil {
				return appdata.Blob{}, err
			}
			return blob, nil
		}
	}
}

func failAwaitedTransfer(cfg ExchangeConfig, transferID, candidatePeer string, candidateErr, fallback error) (appdata.Blob, error) {
	if candidateErr != nil {
		return appdata.Blob{}, failTransfer(cfg.Data, transferID, candidatePeer, candidateErr)
	}
	return appdata.Blob{}, failTransfer(cfg.Data, transferID, candidatePeer, fallback)
}

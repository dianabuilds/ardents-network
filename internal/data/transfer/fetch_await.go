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
				_, _ = cfg.Data.FailTransfer(transferID, candidatePeer, candidateErr.Error())
				return appdata.Blob{}, candidateErr
			}
			err := ctx.Err()
			_, _ = cfg.Data.FailTransfer(transferID, candidatePeer, err.Error())
			return appdata.Blob{}, err
		case payload, ok := <-responses:
			if !ok {
				return failAwaitedTransfer(cfg, transferID, candidatePeer, candidateErr, fmt.Errorf("blob response stream closed"))
			}
			blob, peer, totalBytes, err := acceptBlobResponse(cfg, blobID, requester, requestID, payload)
			if err != nil {
				var terminalErr blobFetchTerminalError
				if errors.As(err, &terminalErr) {
					_, _ = cfg.Data.FailTransfer(transferID, peer, terminalErr.err.Error())
					return appdata.Blob{}, terminalErr.err
				}
				var rememberedErr blobFetchCandidateError
				if errors.As(err, &rememberedErr) {
					candidateErr = rememberedErr.err
					candidatePeer = peer
				}
				continue
			}
			_, _ = cfg.Data.CompleteTransfer(transferID, peer, totalBytes, "blob fetched from trusted peer")
			return blob, nil
		}
	}
}

func failAwaitedTransfer(cfg ExchangeConfig, transferID, candidatePeer string, candidateErr, fallback error) (appdata.Blob, error) {
	if candidateErr != nil {
		_, _ = cfg.Data.FailTransfer(transferID, candidatePeer, candidateErr.Error())
		return appdata.Blob{}, candidateErr
	}
	_, _ = cfg.Data.FailTransfer(transferID, candidatePeer, fallback.Error())
	return appdata.Blob{}, fallback
}

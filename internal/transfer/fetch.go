package transfer

import (
	"context"
	"errors"
	"fmt"

	model "ardents/internal/content/catalog"
	networkprivacy "ardents/internal/messaging"
)

func StartBlobExchange(ctx context.Context, cfg ExchangeConfig) error {
	if cfg.Private == nil {
		return networkprivacy.ChannelGrantUnavailable()
	}
	if err := cfg.Private.Start(ctx); err != nil {
		return err
	}
	go serveBlobRequests(ctx, cfg, cfg.Private.Requests())
	go observePrivateFailures(ctx, cfg)
	return nil
}

func FetchBlob(ctx context.Context, cfg ExchangeConfig, blobID string) (model.Blob, error) {
	if cfg.Identity == nil || cfg.Private == nil || cfg.Data == nil || cfg.History == nil {
		return model.Blob{}, fmt.Errorf("blob fetch dependencies are unavailable")
	}
	requestID, err := requestIdentity("blob-fetch")
	if err != nil {
		return model.Blob{}, err
	}
	requester := cfg.Identity().Principal
	transfer, err := cfg.History.Start(Record{
		ID:         requestID,
		Kind:       "blob_fetch",
		ResourceID: blobID,
		Direction:  "inbound",
		State:      "pending",
		Reason:     "waiting for remote blob response",
	})
	if err != nil {
		return model.Blob{}, err
	}

	responses, unregister, err := cfg.Private.RegisterResponse(requestID)
	if err != nil {
		return model.Blob{}, failTransfer(cfg.History, transfer.ID, "", err)
	}
	defer unregister()
	if err := publishBlobFetchRequest(ctx, cfg, requestID, blobID, requester); err != nil {
		return model.Blob{}, failTransfer(cfg.History, transfer.ID, "", err)
	}
	return awaitBlobFetchResponse(ctx, cfg, transfer.ID, blobID, requester, requestID, responses)
}

func serveBlobRequests(ctx context.Context, cfg ExchangeConfig, reqs <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			return
		case payload, ok := <-reqs:
			if !ok {
				return
			}
			if err := handleBlobRequest(ctx, cfg, payload); err != nil && cfg.RecordEvent != nil {
				eventType := "blob_request_ignored"
				message := "blob request ignored"
				code := "data.blob.request_ignored"
				if outcomeErr, ok := errors.AsType[blobRequestOutcomeError](err); ok && outcomeErr.responded {
					eventType = "blob_request_rejected"
					message = "blob request rejected"
					code = "data.blob.request_rejected"
				}
				cfg.RecordEvent("data", eventType, cfg.ConfigName, message, code, map[string]any{"detail": err.Error()})
			}
		}
	}
}

func handleBlobRequest(ctx context.Context, cfg ExchangeConfig, payload []byte) error {
	if cfg.Identity == nil || cfg.Policy == nil || cfg.Data == nil || cfg.History == nil || cfg.PrivateKey == nil {
		return fmt.Errorf("blob request dependencies are unavailable")
	}

	req, err := decodeBlobRequest(payload)
	if err != nil {
		return err
	}
	if req.Requester == cfg.Identity().Principal {
		return nil
	}
	transfer, err := startServeTransfer(cfg, req)
	if err != nil {
		return err
	}
	wire, responseErr, err := prepareServedBlobResponse(cfg, req)
	if err != nil {
		return failTransfer(cfg.History, transfer.ID, req.Requester, err)
	}
	if wire == nil {
		if responseErr != nil {
			return failTransfer(cfg.History, transfer.ID, req.Requester, responseErr)
		}
		return fmt.Errorf("blob response is unavailable")
	}
	if err := cfg.Private.Publish(ctx, networkprivacy.MessageClassBlobFetchResponse, wire); err != nil {
		if responseErr != nil {
			err = fmt.Errorf("%w: %v", err, responseErr)
		}
		return failTransfer(cfg.History, transfer.ID, req.Requester, err)
	}
	if responseErr != nil {
		return blobRequestOutcomeError{
			err: failTransfer(cfg.History, transfer.ID, req.Requester, responseErr), responded: true,
		}
	}
	return completeTransfer(cfg.History, transfer.ID, req.Requester, 0, "blob response sent to peer")
}

func observePrivateFailures(ctx context.Context, cfg ExchangeConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-cfg.Private.Failures():
			if err != nil && cfg.RecordEvent != nil {
				cfg.RecordEvent("data", "privacy_rejected", cfg.ConfigName, "private data envelope rejected", networkprivacy.CodeOf(err), nil)
			}
		}
	}
}

func startServeTransfer(cfg ExchangeConfig, req blobFetchRequest) (Record, error) {
	return cfg.History.Start(Record{
		ID:         "serve-" + req.RequestID,
		Kind:       "blob_re_serve",
		ResourceID: req.ResourceID,
		Direction:  "outbound",
		State:      "pending",
		Peer:       req.Requester,
		Reason:     "processing peer blob request",
	})
}

func prepareServedBlobResponse(cfg ExchangeConfig, req blobFetchRequest) ([]byte, error, error) {
	key := cfg.PrivateKey()
	if len(key) == 0 {
		return nil, nil, fmt.Errorf("blob responder signing key is unavailable")
	}
	return prepareBlobResponseWire(cfg, cfg.Identity().Principal, key, req)
}

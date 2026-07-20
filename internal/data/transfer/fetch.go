package transfer

import (
	"context"
	"errors"
	"fmt"
	"time"

	appdata "ardents/internal/data"
	networkprivacy "ardents/internal/network/privacy"
)

func StartBlobExchange(ctx context.Context, cfg ExchangeConfig) error {
	if cfg.Private == nil {
		return networkprivacy.CapabilityUnavailable()
	}
	if err := cfg.Private.Start(ctx); err != nil {
		return err
	}
	go serveBlobRequests(ctx, cfg, cfg.Private.Requests())
	go observePrivateFailures(ctx, cfg)
	return nil
}

func FetchBlob(ctx context.Context, cfg ExchangeConfig, blobID string) (appdata.Blob, error) {
	if cfg.Identity == nil || cfg.Private == nil || cfg.Data == nil {
		return appdata.Blob{}, fmt.Errorf("blob fetch dependencies are unavailable")
	}
	requestID := fmt.Sprintf("blob-fetch-%d", time.Now().UTC().UnixNano())
	requester := cfg.Identity.NodeSummary().Principal
	transfer, err := cfg.Data.StartTransfer(appdata.TransferRecord{
		ID:         requestID,
		Kind:       "blob_fetch",
		ResourceID: blobID,
		Direction:  "inbound",
		State:      "pending",
		Reason:     "waiting for remote blob response",
	})
	if err != nil {
		return appdata.Blob{}, err
	}

	responses, unregister, err := cfg.Private.RegisterResponse(requestID)
	if err != nil {
		_, _ = cfg.Data.FailTransfer(transfer.ID, "", err.Error())
		return appdata.Blob{}, err
	}
	defer unregister()
	if err := publishBlobFetchRequest(ctx, cfg, requestID, blobID, requester); err != nil {
		_, _ = cfg.Data.FailTransfer(transfer.ID, "", err.Error())
		return appdata.Blob{}, err
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
			if err := handleBlobRequest(ctx, cfg, payload); err != nil && cfg.Diagnostics != nil {
				eventType := "blob_request_ignored"
				message := "blob request ignored"
				code := "data.blob.request_ignored"
				var outcomeErr blobRequestOutcomeError
				if errors.As(err, &outcomeErr) && outcomeErr.responded {
					eventType = "blob_request_rejected"
					message = "blob request rejected"
					code = "data.blob.request_rejected"
				}
				cfg.Diagnostics.RecordEvent("data", eventType, cfg.ConfigName, message, code, map[string]any{"detail": err.Error()})
			}
		}
	}
}

func handleBlobRequest(ctx context.Context, cfg ExchangeConfig, payload []byte) error {
	if cfg.Identity == nil || cfg.Policy == nil || cfg.Transport == nil || cfg.Data == nil || cfg.PrivateKey == nil {
		return fmt.Errorf("blob request dependencies are unavailable")
	}

	req, err := decodeBlobRequest(payload)
	if err != nil {
		return err
	}
	if req.Requester == cfg.Identity.NodeSummary().Principal {
		return nil
	}
	transfer, err := startServeTransfer(cfg, req)
	if err != nil {
		return err
	}
	wire, responseErr, err := prepareServedBlobResponse(cfg, req)
	if err != nil {
		_, _ = cfg.Data.FailTransfer(transfer.ID, req.Requester, err.Error())
		return err
	}
	if wire == nil {
		if responseErr != nil {
			_, _ = cfg.Data.FailTransfer(transfer.ID, req.Requester, responseErr.Error())
		}
		return responseErr
	}
	if err := cfg.Private.Publish(ctx, networkprivacy.MessageClassBlobFetchResponse, wire); err != nil {
		_, _ = cfg.Data.FailTransfer(transfer.ID, req.Requester, err.Error())
		if responseErr != nil {
			return fmt.Errorf("%w: %v", err, responseErr)
		}
		return err
	}
	if responseErr != nil {
		_, _ = cfg.Data.FailTransfer(transfer.ID, req.Requester, responseErr.Error())
		return blobRequestOutcomeError{err: responseErr, responded: true}
	}
	_, _ = cfg.Data.CompleteTransfer(transfer.ID, req.Requester, 0, "blob response sent to peer")
	return nil
}

func observePrivateFailures(ctx context.Context, cfg ExchangeConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-cfg.Private.Failures():
			if err != nil && cfg.Diagnostics != nil {
				cfg.Diagnostics.RecordEvent("data", "privacy_rejected", cfg.ConfigName, "private data envelope rejected", networkprivacy.CodeOf(err), nil)
			}
		}
	}
}

func startServeTransfer(cfg ExchangeConfig, req blobFetchRequest) (appdata.TransferRecord, error) {
	return cfg.Data.StartTransfer(appdata.TransferRecord{
		ID:         "serve-" + req.RequestID,
		Kind:       "blob_re_serve",
		ResourceID: req.BlobID,
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
	return prepareBlobResponseWire(cfg, cfg.Identity.NodeSummary().Principal, key, req)
}

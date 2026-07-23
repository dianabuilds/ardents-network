package transfer

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/discovery"
	"ardents/internal/storage"
	"encoding/base64"
	"errors"
	"fmt"
)

type blobFetchTerminalError struct {
	err error
}

func (e blobFetchTerminalError) Error() string {
	return e.err.Error()
}

func acceptBlobResponse(cfg ExchangeConfig, blobID, requester, requestID string, payload []byte) (model.Blob, string, int64, error) {
	if cfg.Discovery == nil || cfg.Trust == nil || cfg.Data == nil {
		return model.Blob{}, "", 0, fmt.Errorf("blob response dependencies are unavailable")
	}

	response, err := decodeBlobResponse(payload, blobID, requester, requestID)
	if err != nil {
		return model.Blob{}, "", 0, err
	}
	entry, outcome, ok := cfg.Discovery.Resolve(response.Source, "node")
	if !ok || outcome != "found" {
		return model.Blob{}, response.Source, 0, blobFetchCandidateError{err: fmt.Errorf("remote source is undiscoverable")}
	}
	trustResult := cfg.Trust.Evaluate(entry.Record)
	if !trustResult.Usable {
		return model.Blob{}, response.Source, 0, blobFetchCandidateError{err: fmt.Errorf("remote source is not trusted")}
	}
	if err := verifyBlobResponder(response, entry.Record); err != nil {
		return model.Blob{}, response.Source, 0, blobFetchCandidateError{err: err}
	}
	if response.Status == blobFetchStatusError {
		detail := response.Error
		if detail == "" {
			detail = "blob fetch rejected"
		}
		return model.Blob{}, response.Source, 0, blobFetchTerminalError{err: errors.New(detail)}
	}
	return storeAcceptedBlobResponse(cfg, response, trustResult)
}

func storeAcceptedBlobResponse(cfg ExchangeConfig, response blobFetchResponse, trustResult discovery.TrustResult) (model.Blob, string, int64, error) {
	raw, err := base64.StdEncoding.DecodeString(response.Payload)
	if err != nil {
		return model.Blob{}, response.Source, 0, err
	}
	response.Blob.State = "available-local"
	if response.Blob.Retention == "" {
		response.Blob.Retention = "fetched"
	}
	stored, err := cfg.Data.StoreBlob(*response.Blob, raw)
	if err != nil {
		return model.Blob{}, response.Source, 0, err
	}
	_, err = cfg.Data.ObserveBlobSource(stored.Reference.String(), model.BlobSourceRecord{
		NodeID:    response.Source,
		Trust:     model.SourceTrust{State: discovery.TrustStateForResult(trustResult), Outcome: trustResult.Outcome, Valid: trustResult.Valid, Trusted: trustResult.Trusted, Usable: trustResult.Usable},
		Usable:    trustResult.Usable,
		Transport: "remote",
		Reason:    "trusted remote source answered blob fetch",
	})
	if err != nil {
		return model.Blob{}, response.Source, 0, err
	}
	if cfg.Publish != nil {
		cfg.Publish("data.blob_fetched", map[string]any{"reference": stored.Reference.String(), "source": response.Source})
	}
	if cfg.RecordEvent != nil {
		cfg.RecordEvent("data", "blob_fetched", stored.Reference.String(), "blob fetched from trusted peer", "", map[string]any{"source": response.Source})
	}
	return stored, response.Source, int64(len(raw)), nil
}

func decodeBlobResponse(payload []byte, blobID, requester, requestID string) (blobFetchResponse, error) {
	var response blobFetchResponse
	if err := storage.DecodeJSONStrict(payload, &response); err != nil {
		return blobFetchResponse{}, err
	}
	if response.RequestID != requestID || response.Requester != requester || response.Source == "" {
		return blobFetchResponse{}, fmt.Errorf("blob response does not match request")
	}
	if response.Status == "" {
		response.Status = blobFetchStatusOK
	}
	switch response.Status {
	case blobFetchStatusOK:
		if response.Blob == nil || response.Blob.Reference.String() != blobID {
			return blobFetchResponse{}, fmt.Errorf("blob response does not match request")
		}
	case blobFetchStatusError:
	default:
		return blobFetchResponse{}, fmt.Errorf("blob response status is invalid")
	}
	return response, nil
}

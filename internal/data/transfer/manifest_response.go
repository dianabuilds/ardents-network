package transfer

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	appdata "ardents/internal/data"
)

func prepareManifestResponseWire(cfg ExchangeConfig, source string, key ed25519.PrivateKey, req blobFetchRequest) ([]byte, error, error) {
	manifest, err := prepareManifestResponse(cfg, req)
	if err != nil {
		if err.Error() == "manifest not found" {
			return nil, err, nil
		}
		wire, marshalErr := marshalManifestResponse(source, key, req, appdata.Manifest{}, err)
		return wire, err, marshalErr
	}
	wire, err := marshalManifestResponse(source, key, req, manifest, nil)
	return wire, nil, err
}

func prepareManifestResponse(cfg ExchangeConfig, req blobFetchRequest) (appdata.Manifest, error) {
	manifest, ok := cfg.Data.GetManifest(req.BlobID)
	if !ok {
		return appdata.Manifest{}, fmt.Errorf("manifest not found")
	}
	if !manifest.Encrypted || (manifest.Kind != "chunk-leaf" && manifest.Kind != "chunk-root") {
		return appdata.Manifest{}, fmt.Errorf("manifest is not eligible for private transfer")
	}
	if err := verifyBlobRequester(req); err != nil {
		return appdata.Manifest{}, err
	}
	return manifest, nil
}

func marshalManifestResponse(source string, key ed25519.PrivateKey, req blobFetchRequest, manifest appdata.Manifest, responseErr error) ([]byte, error) {
	status := blobFetchStatusOK
	detail := ""
	var value *appdata.Manifest
	if responseErr != nil {
		status = blobFetchStatusError
		detail = responseErr.Error()
	} else {
		copy := manifest
		value = &copy
	}
	response := blobFetchResponse{
		RequestID: req.RequestID, Requester: req.Requester, BlobID: req.BlobID,
		ResourceKind: "manifest", Status: status, Error: detail, Manifest: value, Source: source,
	}
	signed, err := canonicalBlobFetchResponse(response)
	if err != nil {
		return nil, err
	}
	response.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, signed))
	return json.Marshal(response)
}

func acceptManifestResponse(cfg ExchangeConfig, manifestID, requester, requestID string, payload []byte) (appdata.Manifest, string, error) {
	response, err := decodeManifestResponse(payload, manifestID, requester, requestID)
	if err != nil {
		return appdata.Manifest{}, "", err
	}
	entry, outcome, ok := cfg.Discovery.Resolve(response.Source, "node")
	if !ok || outcome != "found" {
		return appdata.Manifest{}, response.Source, blobFetchCandidateError{err: fmt.Errorf("remote source is undiscoverable")}
	}
	trustResult := cfg.Trust.Evaluate(entry.Record)
	if !trustResult.Usable {
		return appdata.Manifest{}, response.Source, blobFetchCandidateError{err: fmt.Errorf("remote source is not trusted")}
	}
	if err := verifyBlobResponder(response, entry.Record); err != nil {
		return appdata.Manifest{}, response.Source, blobFetchCandidateError{err: err}
	}
	if response.Status == blobFetchStatusError {
		return appdata.Manifest{}, response.Source, blobFetchTerminalError{err: errors.New(response.Error)}
	}
	return *response.Manifest, response.Source, nil
}

func decodeManifestResponse(payload []byte, manifestID, requester, requestID string) (blobFetchResponse, error) {
	var response blobFetchResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return blobFetchResponse{}, err
	}
	if response.RequestID != requestID || response.Requester != requester || response.BlobID != manifestID || response.ResourceKind != "manifest" || response.Source == "" {
		return blobFetchResponse{}, fmt.Errorf("manifest response does not match request")
	}
	if response.Status == blobFetchStatusOK {
		if response.Manifest == nil || response.Manifest.ID != manifestID {
			return blobFetchResponse{}, fmt.Errorf("manifest response does not match request")
		}
		return response, nil
	}
	if response.Status != blobFetchStatusError || response.Error == "" {
		return blobFetchResponse{}, fmt.Errorf("manifest response status is invalid")
	}
	return response, nil
}

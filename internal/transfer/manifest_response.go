package transfer

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
	"ardents/internal/storage"
)

func prepareManifestResponseWire(cfg ExchangeConfig, source string, key ed25519.PrivateKey, req blobFetchRequest) ([]byte, error, error) {
	manifest, err := prepareManifestResponse(cfg, req)
	if err != nil {
		if err.Error() == "manifest not found" {
			return nil, err, nil
		}
		wire, marshalErr := marshalManifestResponse(source, key, req, model.Manifest{}, err)
		return wire, err, marshalErr
	}
	wire, err := marshalManifestResponse(source, key, req, manifest, nil)
	return wire, nil, err
}

func prepareManifestResponse(cfg ExchangeConfig, req blobFetchRequest) (model.Manifest, error) {
	manifest, ok := readTransferManifest(cfg.Data, req.Owner, req.ResourceID)
	if !ok {
		return model.Manifest{}, fmt.Errorf("manifest not found")
	}
	if !manifest.Encrypted || (manifest.Kind != "chunk-leaf" && manifest.Kind != "chunk-root") {
		return model.Manifest{}, fmt.Errorf("manifest is not eligible for private transfer")
	}
	if err := verifyBlobRequester(req); err != nil {
		return model.Manifest{}, err
	}
	return manifest, nil
}

func marshalManifestResponse(source string, key ed25519.PrivateKey, req blobFetchRequest, manifest model.Manifest, responseErr error) ([]byte, error) {
	status := blobFetchStatusOK
	detail := ""
	var value *manifestWire
	if responseErr != nil {
		status = blobFetchStatusError
		detail = responseErr.Error()
	} else {
		value = new(manifestWireFromSnapshot(manifest))
	}
	response := blobFetchResponse{
		RequestID: req.RequestID, Requester: req.Requester,
		ResourceKind: "manifest", Status: status, Error: detail, Manifest: value, Source: source,
	}
	signed, err := canonicalBlobFetchResponse(response)
	if err != nil {
		return nil, err
	}
	response.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, signed))
	return json.Marshal(response)
}

func acceptManifestResponse(cfg ExchangeConfig, owner principal.ID, manifestID, requester, requestID string, payload []byte) (model.Manifest, string, error) {
	response, err := decodeManifestResponse(payload, manifestID, requester, requestID)
	if err != nil {
		return model.Manifest{}, "", err
	}
	entry, outcome, ok := cfg.Discovery.Resolve(response.Source, "node")
	if !ok || outcome != "found" {
		return model.Manifest{}, response.Source, blobFetchCandidateError{err: fmt.Errorf("remote source is undiscoverable")}
	}
	trustResult := cfg.Trust.Evaluate(entry.Record)
	if !trustResult.Usable {
		return model.Manifest{}, response.Source, blobFetchCandidateError{err: fmt.Errorf("remote source is not trusted")}
	}
	if err := verifyBlobResponder(response, entry.Record); err != nil {
		return model.Manifest{}, response.Source, blobFetchCandidateError{err: err}
	}
	if response.Status == blobFetchStatusError {
		return model.Manifest{}, response.Source, blobFetchTerminalError{err: errors.New(response.Error)}
	}
	manifest, err := manifestFromWire(*response.Manifest)
	if err != nil {
		return model.Manifest{}, response.Source, blobFetchCandidateError{err: err}
	}
	if !manifest.Owner.Equal(owner) {
		return model.Manifest{}, response.Source, blobFetchCandidateError{err: fmt.Errorf("manifest response owner does not match request")}
	}
	return manifest, response.Source, nil
}

func readTransferManifest(data DataExchange, owner, id string) (model.Manifest, bool) {
	if owner == "" {
		return model.Manifest{}, false
	}
	parsed, err := principal.Parse(owner)
	if err != nil {
		return model.Manifest{}, false
	}
	return data.ReadTransferManifest(parsed, id)
}

func manifestWireFromSnapshot(in model.Manifest) manifestWire {
	refs := make([]refWire, 0, len(in.Refs))
	for _, ref := range in.Refs {
		refs = append(refs, refWire{Kind: ref.Kind, ID: ref.ID})
	}
	return manifestWire{
		ID: in.ID, Kind: in.Kind, Owner: in.Owner.String(), Refs: refs,
		Access: in.Access, Retention: in.Retention, Encrypted: in.Encrypted,
		Metadata: cloneMetadata(in.Metadata), CreatedAt: in.CreatedAt,
	}
}

func manifestFromWire(in manifestWire) (model.Manifest, error) {
	owner, err := principal.Parse(in.Owner)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("remote manifest owner is invalid")
	}
	refs := make([]model.Ref, 0, len(in.Refs))
	for _, ref := range in.Refs {
		refs = append(refs, model.Ref{Kind: ref.Kind, ID: ref.ID})
	}
	return model.Manifest{
		ID: in.ID, Kind: in.Kind, Owner: owner, Refs: refs,
		Access: in.Access, Retention: in.Retention, Encrypted: in.Encrypted,
		Metadata: cloneMetadata(in.Metadata), CreatedAt: in.CreatedAt,
	}, nil
}

func cloneMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}

func decodeManifestResponse(payload []byte, manifestID, requester, requestID string) (blobFetchResponse, error) {
	var response blobFetchResponse
	if err := storage.DecodeJSONStrict(payload, &response); err != nil {
		return blobFetchResponse{}, err
	}
	if response.RequestID != requestID || response.Requester != requester || response.ResourceKind != "manifest" || response.Source == "" {
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

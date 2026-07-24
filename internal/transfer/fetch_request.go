package transfer

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"ardents/internal/content"
	model "ardents/internal/content/catalog"
	identityprincipal "ardents/internal/identity/principal"
	networkprivacy "ardents/internal/messaging"
	"ardents/internal/storage"
)

func prepareBlobResponseWire(cfg ExchangeConfig, source string, key ed25519.PrivateKey, req blobFetchRequest) ([]byte, error, error) {
	if req.ResourceKind == "manifest" {
		return prepareManifestResponseWire(cfg, source, key, req)
	}
	if req.ResourceKind != "" && req.ResourceKind != "blob" {
		return nil, nil, fmt.Errorf("data fetch resource kind is unsupported")
	}
	blob, payload, err := prepareBlobResponse(cfg, req)
	if err != nil {
		if !shouldReplyWithBlobFetchError(err) {
			return nil, err, nil
		}
		wire, marshalErr := marshalBlobErrorResponse(source, key, req, err)
		if marshalErr != nil {
			return nil, nil, marshalErr
		}
		return wire, err, nil
	}
	wire, err := marshalBlobResponse(source, key, req, blob, payload)
	if err != nil {
		return nil, nil, err
	}
	return wire, nil, nil
}

func prepareBlobResponse(cfg ExchangeConfig, req blobFetchRequest) (model.Blob, []byte, error) {
	if _, err := model.ParseContentReference(req.ResourceID); err != nil {
		return model.Blob{}, nil, fmt.Errorf("blob Content Reference is invalid")
	}
	blob, ok := cfg.Data.GetBlob(req.ResourceID)
	if !ok {
		return model.Blob{}, nil, fmt.Errorf("blob not found")
	}
	if blob.Retention == "staging" {
		return model.Blob{}, nil, fmt.Errorf("blob is not committed for serving")
	}
	blobView := blobPolicyView(blob)
	if err := authorizeBlobServing(cfg, req.ResourceID, blobView); err != nil {
		if cfg.PolicyDenied != nil {
			cfg.PolicyDenied(req.ResourceID, "data.blob_re_serve", err)
		}
		return model.Blob{}, nil, err
	}
	if err := authorizeBlobRequester(req, blobView); err != nil {
		return model.Blob{}, nil, err
	}
	raw, err := cfg.Data.GetBlobPayload(req.ResourceID)
	if err != nil {
		return model.Blob{}, nil, err
	}
	return blob, raw, nil
}

func authorizeBlobServing(cfg ExchangeConfig, blobID string, blob content.BlobPolicyView) error {
	if cfg.Replicas != nil && cfg.Replicas.HasCurrentReplicaCommitment(blobID) {
		return cfg.Policy.AllowReplicaBlobServing(blob)
	}
	return cfg.Policy.AllowPeerBlobReserving(blob)
}

func decodeBlobRequest(payload []byte) (blobFetchRequest, error) {
	var req blobFetchRequest
	if err := storage.DecodeJSONStrict(payload, &req); err != nil {
		return blobFetchRequest{}, err
	}
	if req.RequestID == "" || req.ResourceID == "" || req.Requester == "" {
		return blobFetchRequest{}, fmt.Errorf("blob request is incomplete")
	}
	if req.ResourceKind == "manifest" {
		if _, err := identityprincipal.Parse(req.Owner); err != nil {
			return blobFetchRequest{}, fmt.Errorf("manifest request owner is invalid")
		}
	}
	return req, nil
}

func publishBlobFetchRequest(ctx context.Context, cfg ExchangeConfig, requestID, blobID, requester string) error {
	return publishDataFetchRequest(ctx, cfg, requestID, blobID, requester, "")
}

func publishDataFetchRequest(ctx context.Context, cfg ExchangeConfig, requestID, resourceID, requester, resourceKind string) error {
	return publishOwnedDataFetchRequest(ctx, cfg, requestID, resourceID, requester, resourceKind, "")
}

func publishOwnedDataFetchRequest(ctx context.Context, cfg ExchangeConfig, requestID, resourceID, requester, resourceKind, owner string) error {
	if cfg.Identity == nil || cfg.Private == nil || cfg.PrivateKey == nil {
		return fmt.Errorf("blob requester dependencies are unavailable")
	}
	if resourceKind == "manifest" {
		if _, err := identityprincipal.Parse(owner); err != nil {
			return fmt.Errorf("manifest request owner is invalid")
		}
	}

	req := blobFetchRequest{
		RequestID:    requestID,
		ResourceID:   resourceID,
		ResourceKind: resourceKind,
		Owner:        owner,
		Requester:    requester,
		PublicKey:    cfg.Identity().PublicKey,
	}
	payload, err := canonicalBlobFetchRequest(req)
	if err != nil {
		return err
	}
	key := cfg.PrivateKey()
	if len(key) == 0 {
		return fmt.Errorf("blob requester signing key is unavailable")
	}
	req.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
	wire, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return cfg.Private.Publish(ctx, networkprivacy.MessageClassBlobFetchRequest, wire)
}

func authorizeBlobRequester(req blobFetchRequest, blob content.BlobPolicyView) error {
	if !blob.Encrypted {
		return fmt.Errorf("plaintext blob re-serve is not allowed")
	}
	return verifyBlobRequester(req)
}

func blobPolicyView(blob model.Blob) content.BlobPolicyView {
	return content.BlobPolicyView{State: blob.State, Retention: blob.Retention, Encrypted: blob.Encrypted}
}

func shouldReplyWithBlobFetchError(err error) bool {
	return err != nil && err.Error() != "blob not found"
}

func canonicalBlobFetchRequest(req blobFetchRequest) ([]byte, error) {
	return json.Marshal(struct {
		RequestID    string `json:"request_id"`
		ResourceID   string `json:"resource_id"`
		ResourceKind string `json:"resource_kind,omitempty"`
		Owner        string `json:"owner,omitempty"`
		Requester    string `json:"requester"`
		PublicKey    string `json:"public_key"`
	}{
		RequestID:    req.RequestID,
		ResourceID:   req.ResourceID,
		ResourceKind: req.ResourceKind,
		Owner:        req.Owner,
		Requester:    req.Requester,
		PublicKey:    req.PublicKey,
	})
}

func verifyBlobRequester(req blobFetchRequest) error {
	if req.PublicKey == "" || req.Signature == "" {
		return fmt.Errorf("blob requester identity is incomplete")
	}
	expectedRequester, err := identityprincipal.FromPublicKey(req.PublicKey)
	if err != nil {
		return fmt.Errorf("blob requester identity is invalid")
	}
	if expectedRequester != req.Requester {
		return fmt.Errorf("blob requester identity does not match principal")
	}
	publicKey, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		return fmt.Errorf("blob requester public key is invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("blob requester signature is invalid")
	}
	payload, err := canonicalBlobFetchRequest(req)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("blob requester signature verification failed")
	}
	return nil
}

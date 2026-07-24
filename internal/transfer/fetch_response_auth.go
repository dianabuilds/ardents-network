package transfer

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"

	model "ardents/internal/content/catalog"
	"ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"
)

const (
	blobFetchResponseVersion = 2
	blobFetchStatusOK        = "ok"
	blobFetchStatusError     = "error"
)

func marshalBlobResponse(source string, key ed25519.PrivateKey, req blobFetchRequest, blob model.Blob, payload []byte) ([]byte, error) {
	response := blobFetchResponse{
		Version:      blobFetchResponseVersion,
		RequestID:    req.RequestID,
		Requester:    req.Requester,
		ResourceID:   req.ResourceID,
		ResourceKind: req.ResourceKind,
		Owner:        req.Owner,
		Status:       blobFetchStatusOK,
		Blob:         &blob,
		Payload:      base64.StdEncoding.EncodeToString(payload),
		Source:       source,
	}
	signed, err := canonicalBlobFetchResponse(response)
	if err != nil {
		return nil, err
	}
	response.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, signed))
	return json.Marshal(response)
}

func marshalBlobErrorResponse(source string, key ed25519.PrivateKey, req blobFetchRequest, err error) ([]byte, error) {
	response := blobFetchResponse{
		Version:      blobFetchResponseVersion,
		RequestID:    req.RequestID,
		Requester:    req.Requester,
		ResourceID:   req.ResourceID,
		ResourceKind: req.ResourceKind,
		Owner:        req.Owner,
		Status:       blobFetchStatusError,
		Error:        err.Error(),
		Source:       source,
	}
	signed, signErr := canonicalBlobFetchResponse(response)
	if signErr != nil {
		return nil, signErr
	}
	response.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, signed))
	return json.Marshal(response)
}

func canonicalBlobFetchResponse(response blobFetchResponse) ([]byte, error) {
	return json.Marshal(struct {
		Version      uint32        `json:"version"`
		RequestID    string        `json:"request_id"`
		Requester    string        `json:"requester"`
		ResourceID   string        `json:"resource_id"`
		ResourceKind string        `json:"resource_kind,omitempty"`
		Owner        string        `json:"owner,omitempty"`
		Status       string        `json:"status,omitempty"`
		Error        string        `json:"error,omitempty"`
		Blob         *model.Blob   `json:"blob,omitempty"`
		Manifest     *manifestWire `json:"manifest,omitempty"`
		Payload      string        `json:"payload"`
		Source       string        `json:"source"`
	}{
		Version:      response.Version,
		RequestID:    response.RequestID,
		Requester:    response.Requester,
		ResourceID:   response.ResourceID,
		ResourceKind: response.ResourceKind,
		Owner:        response.Owner,
		Status:       response.Status,
		Error:        response.Error,
		Blob:         response.Blob,
		Manifest:     response.Manifest,
		Payload:      response.Payload,
		Source:       response.Source,
	})
}

func verifyBlobResponder(response blobFetchResponse, record discovery.Record) error {
	if response.Signature == "" {
		return fmt.Errorf("blob response signature is missing")
	}
	expectedSource, err := identityprincipal.FromPublicKey(record.PublicKeyText())
	if err != nil {
		return fmt.Errorf("blob response source identity is invalid")
	}
	if expectedSource != response.Source {
		return fmt.Errorf("blob response source does not match signing identity")
	}
	publicKey, err := base64.StdEncoding.DecodeString(record.PublicKeyText())
	if err != nil {
		return fmt.Errorf("blob response public key is invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(response.Signature)
	if err != nil {
		return fmt.Errorf("blob response signature is invalid")
	}
	payload, err := canonicalBlobFetchResponse(response)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("blob response signature verification failed")
	}
	return nil
}

package transfer

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"ardents/internal/content"
	model "ardents/internal/content/catalog"
	"ardents/internal/discovery"
	"ardents/internal/identity/principal"
)

func requestIdentity(prefix string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate %s request identity: %w", prefix, err)
	}
	return prefix + "-" + hex.EncodeToString(random), nil
}

func completeTransfer(history History, id, peer string, totalBytes int64, reason string) error {
	if _, err := history.Complete(id, peer, totalBytes, reason); err != nil {
		return fmt.Errorf("record completed transfer %s: %w", id, err)
	}
	return nil
}

func failTransfer(history History, id, peer string, cause error) error {
	if cause == nil {
		cause = fmt.Errorf("transfer failed without a cause")
	}
	if _, err := history.Fail(id, peer, cause.Error()); err != nil {
		return errors.Join(cause, fmt.Errorf("record failed transfer %s: %w", id, err))
	}
	return cause
}

func updateTransferProgress(history History, id string, progressBytes, totalBytes int64, reason string) error {
	if _, err := history.Progress(id, progressBytes, totalBytes, reason); err != nil {
		return fmt.Errorf("record transfer progress %s: %w", id, err)
	}
	return nil
}

type IdentitySummary struct {
	Principal string
	PublicKey string
}

type PolicyService interface {
	AllowPeerBlobReserving(content.BlobPolicyView) error
	AllowReplicaBlobServing(content.BlobPolicyView) error
}

type DataExchange interface {
	GetBlob(string) (model.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
	ReadTransferManifest(principal.ID, string) (model.Manifest, bool)
	StoreBlob(model.Blob, []byte) (model.Blob, error)
	WriteTransferManifest(model.Manifest) (model.Manifest, error)
	ObserveBlobSource(string, model.BlobSourceRecord) (model.BlobSourceRecord, error)
}

type History interface {
	Start(Record) (Record, error)
	Progress(string, int64, int64, string) (Record, error)
	Complete(string, string, int64, string) (Record, error)
	Fail(string, string, string) (Record, error)
}

type ReplicaState interface {
	HasCurrentReplicaCommitment(string) bool
}

type ExchangeConfig struct {
	ConfigName   string
	RecordEvent  func(string, string, string, string, string, map[string]any)
	Discovery    *discovery.Service
	Identity     func() IdentitySummary
	Trust        *discovery.TrustEvaluator
	Policy       PolicyService
	Data         DataExchange
	History      History
	Replicas     ReplicaState
	PrivateKey   func() ed25519.PrivateKey
	Publish      func(string, map[string]any)
	PolicyDenied func(string, string, error)
	Private      *PrivateExchange
	FetchTimeout time.Duration
}

type blobFetchRequest struct {
	RequestID    string `json:"request_id"`
	ResourceID   string `json:"resource_id"`
	ResourceKind string `json:"resource_kind,omitempty"`
	Owner        string `json:"owner,omitempty"`
	Requester    string `json:"requester"`
	PublicKey    string `json:"public_key"`
	Signature    string `json:"signature"`
}

type blobFetchResponse struct {
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
	Signature    string        `json:"signature"`
}

type manifestWire struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Owner     string         `json:"owner"`
	Refs      []refWire      `json:"refs"`
	Access    string         `json:"access"`
	Retention string         `json:"retention"`
	Encrypted bool           `json:"encrypted"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

type refWire struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type blobRequestOutcomeError struct {
	err       error
	responded bool
}

func (e blobRequestOutcomeError) Error() string {
	return e.err.Error()
}

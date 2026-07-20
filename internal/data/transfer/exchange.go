package transfer

import (
	"crypto/ed25519"

	appdata "ardents/internal/data"
	model "ardents/internal/data/model"
	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	policyapi "ardents/internal/policy/api"
)

type DataExchange interface {
	GetBlob(string) (appdata.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
	HasCurrentReplicaCommitment(string) bool
	GetManifest(string) (appdata.Manifest, bool)
	StoreBlob(appdata.Blob, []byte) (appdata.Blob, error)
	PublishManifest(appdata.Manifest) (appdata.Manifest, error)
	ObserveBlobSource(string, appdata.BlobSourceRecord) (appdata.BlobSourceRecord, error)
	StartTransfer(appdata.TransferRecord) (appdata.TransferRecord, error)
	UpdateTransferProgress(string, int64, int64, string) (appdata.TransferRecord, error)
	CompleteTransfer(string, string, int64, string) (appdata.TransferRecord, error)
	FailTransfer(string, string, string) (appdata.TransferRecord, error)
}

type ExchangeConfig struct {
	ConfigName   string
	Diagnostics  *diagnostics.Recorder
	Discovery    *discovery.Service
	Identity     identityapi.Service
	Trust        *discovery.TrustEvaluator
	Transport    transport.Service
	Policy       policyapi.Service
	Data         DataExchange
	PrivateKey   func() ed25519.PrivateKey
	Publish      func(string, map[string]any)
	PolicyDenied func(string, string, error)
	Private      *PrivateExchange
}

type blobFetchRequest struct {
	RequestID    string `json:"request_id"`
	BlobID       string `json:"blob_id"`
	ResourceKind string `json:"resource_kind,omitempty"`
	Requester    string `json:"requester"`
	PublicKey    string `json:"public_key"`
	Signature    string `json:"signature"`
}

type blobFetchResponse struct {
	RequestID    string          `json:"request_id"`
	Requester    string          `json:"requester"`
	BlobID       string          `json:"blob_id"`
	ResourceKind string          `json:"resource_kind,omitempty"`
	Status       string          `json:"status,omitempty"`
	Error        string          `json:"error,omitempty"`
	Blob         model.Blob      `json:"blob"`
	Manifest     *model.Manifest `json:"manifest,omitempty"`
	Payload      string          `json:"payload"`
	Source       string          `json:"source"`
	Signature    string          `json:"signature"`
}

type blobRequestOutcomeError struct {
	err       error
	responded bool
}

func (e blobRequestOutcomeError) Error() string {
	return e.err.Error()
}

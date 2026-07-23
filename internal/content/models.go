package content

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
	"maps"
	"sort"
	"time"
)

type Config struct {
	Now                      func() time.Time
	DefaultLocalRetentionTTL time.Duration
	DefaultRelayRetentionTTL time.Duration
	MaxRelayRetentionBytes   int64
	MaxReplicaRetentionBytes int64
	MaxLocalStorageBytes     int64
	DefaultDesiredReplicas   int
	DefaultMinimumReplicas   int
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}

func sortedKeys[V any](items map[string]V) []string {
	out := make([]string, 0, len(items))
	for id := range items {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func objectModel(in Object) model.Object {
	return model.Object{
		ID: in.ID, Type: in.Type, Owner: in.Owner, Body: cloneMap(in.Body),
		BlobRefs: refModels(in.BlobRefs), CreatedAt: in.CreatedAt,
	}
}

func objectSnapshot(in model.Object) Object {
	return Object{
		ID: in.ID, Type: in.Type, Owner: in.Owner, Body: cloneMap(in.Body),
		BlobRefs: refSnapshots(in.BlobRefs), CreatedAt: in.CreatedAt,
	}
}

func manifestModel(in Manifest) model.Manifest {
	return model.Manifest{
		ID: in.ID, Kind: in.Kind, Owner: in.Owner, Refs: refModels(in.Refs),
		Access: in.Access, Retention: in.Retention, Encrypted: in.Encrypted,
		Metadata: cloneMap(in.Metadata), CreatedAt: in.CreatedAt,
	}
}

func manifestSnapshot(in model.Manifest) Manifest {
	return Manifest{
		ID: in.ID, Kind: in.Kind, Owner: in.Owner, Refs: refSnapshots(in.Refs),
		Access: in.Access, Retention: in.Retention, Encrypted: in.Encrypted,
		Metadata: cloneMap(in.Metadata), CreatedAt: in.CreatedAt,
	}
}

func refModels(in []Ref) []model.Ref {
	out := make([]model.Ref, 0, len(in))
	for _, ref := range in {
		out = append(out, model.Ref{Kind: ref.Kind, ID: ref.ID})
	}
	return out
}

func refSnapshots(in []model.Ref) []Ref {
	out := make([]Ref, 0, len(in))
	for _, ref := range in {
		out = append(out, Ref{Kind: ref.Kind, ID: ref.ID})
	}
	return out
}

const BlobCipherAES256GCM = "aes-256-gcm"

type Ref struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Object struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Owner     principal.ID   `json:"owner"`
	Body      map[string]any `json:"body,omitempty"`
	BlobRefs  []Ref          `json:"blob_refs,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Blob = model.Blob

type PublishBlobCommand struct {
	Blob    Blob   `json:"blob"`
	Payload []byte `json:"payload,omitempty"`
}
type SourceTrust = model.SourceTrust
type BlobSourceRecord = model.BlobSourceRecord
type Manifest struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Owner     principal.ID   `json:"owner"`
	Refs      []Ref          `json:"refs,omitempty"`
	Access    string         `json:"access,omitempty"`
	Retention string         `json:"retention,omitempty"`
	Encrypted bool           `json:"encrypted"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type ChunkFetchResult struct {
	Root         Manifest `json:"root"`
	ChunkCount   int      `json:"chunk_count"`
	FetchedCount int      `json:"fetched_count"`
	ResumedCount int      `json:"resumed_count"`
	TotalBytes   int64    `json:"total_bytes"`
}
type Inventory = model.Inventory
type BlobSource interface {
	GetBlob(string) (Blob, bool)
	GetBlobPayload(string) ([]byte, error)
}

type BlobPolicyView struct {
	Reference model.ContentReference
	MediaType string
	Size      int64
	Hash      string
	Cipher    string
	KeyID     string
	State     string
	Retention string
	Encrypted bool
	ExpiresAt time.Time
	CreatedAt time.Time
}

type RetentionAuthorizer func(blob BlobPolicyView, relay bool, expiresAt time.Time) error

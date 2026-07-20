package api

import (
	nodeapi "ardents/internal/node/api"
	"time"
)

type RefSnapshot struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id,omitempty"`
}

type ObjectSnapshot struct {
	ID        string         `json:"id,omitempty"`
	Type      string         `json:"type,omitempty"`
	Owner     string         `json:"owner,omitempty"`
	Body      map[string]any `json:"body,omitempty"`
	BlobRefs  []RefSnapshot  `json:"blob_refs,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

type BlobSnapshot struct {
	ID        string    `json:"id,omitempty"`
	CID       string    `json:"cid,omitempty"`
	MediaType string    `json:"media_type,omitempty"`
	Size      int64     `json:"size,omitempty"`
	Payload   []byte    `json:"payload,omitempty"`
	Hash      string    `json:"hash,omitempty"`
	Cipher    string    `json:"cipher,omitempty"`
	KeyID     string    `json:"key_id,omitempty"`
	State     string    `json:"state,omitempty"`
	Retention string    `json:"retention,omitempty"`
	Encrypted bool      `json:"encrypted,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type ManifestSnapshot struct {
	ID        string         `json:"id,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	Owner     string         `json:"owner,omitempty"`
	Access    string         `json:"access,omitempty"`
	Retention string         `json:"retention,omitempty"`
	Encrypted bool           `json:"encrypted,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Refs      []RefSnapshot  `json:"refs,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

type ChunkFetchSnapshot struct {
	Root         ManifestSnapshot `json:"root"`
	ChunkCount   int              `json:"chunk_count"`
	FetchedCount int              `json:"fetched_count"`
	ResumedCount int              `json:"resumed_count"`
	TotalBytes   int64            `json:"total_bytes"`
}

type PartSnapshot struct {
	State  string `json:"state,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type DataInventorySnapshot struct {
	Objects            int   `json:"objects,omitempty"`
	Manifests          int   `json:"manifests,omitempty"`
	Blobs              int   `json:"blobs,omitempty"`
	LocalBlobs         int   `json:"local_blobs,omitempty"`
	RemoteBlobs        int   `json:"remote_blobs,omitempty"`
	RetainedTemporary  int   `json:"retained_temporary,omitempty"`
	RelayRetained      int   `json:"relay_retained,omitempty"`
	Pinned             int   `json:"pinned,omitempty"`
	Expired            int   `json:"expired,omitempty"`
	Deleted            int   `json:"deleted,omitempty"`
	Encrypted          int   `json:"encrypted,omitempty"`
	AvailableForResend int   `json:"available_for_resend,omitempty"`
	LocalBytes         int64 `json:"local_bytes,omitempty"`
	RelayBytes         int64 `json:"relay_bytes,omitempty"`
}

type BlobSourceSnapshot struct {
	BlobID     string                `json:"blob_id,omitempty"`
	NodeID     string                `json:"node_id,omitempty"`
	ServiceID  string                `json:"service_id,omitempty"`
	Trust      nodeapi.TrustSnapshot `json:"trust"`
	Usable     bool                  `json:"usable,omitempty"`
	Transport  string                `json:"transport,omitempty"`
	LastSeenAt time.Time             `json:"last_seen_at,omitempty"`
	Reason     string                `json:"reason,omitempty"`
}

type TransferSnapshot struct {
	ID            string     `json:"id,omitempty"`
	Kind          string     `json:"kind,omitempty"`
	ResourceID    string     `json:"resource_id,omitempty"`
	Direction     string     `json:"direction,omitempty"`
	State         string     `json:"state,omitempty"`
	ProgressBytes int64      `json:"progress_bytes,omitempty"`
	TotalBytes    int64      `json:"total_bytes,omitempty"`
	Peer          string     `json:"peer,omitempty"`
	StartedAt     time.Time  `json:"started_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Reason        string     `json:"reason,omitempty"`
}

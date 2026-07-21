// Package catalog owns object and manifest metadata graph.
// It does not own payload bytes or remote transfer.
package catalog

import "time"

type SourceTrust struct {
	State   string `json:"state,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Valid   bool   `json:"valid"`
	Trusted bool   `json:"trusted"`
	Usable  bool   `json:"usable"`
}

type BlobSourceRecord struct {
	BlobID     string      `json:"blob_id"`
	NodeID     string      `json:"node_id,omitempty"`
	ServiceID  string      `json:"service_id,omitempty"`
	Trust      SourceTrust `json:"trust"`
	Usable     bool        `json:"usable"`
	Transport  string      `json:"transport,omitempty"`
	LastSeenAt time.Time   `json:"last_seen_at"`
	Reason     string      `json:"reason,omitempty"`
}

type Inventory struct {
	Objects            int   `json:"objects"`
	Manifests          int   `json:"manifests"`
	Blobs              int   `json:"blobs"`
	LocalBlobs         int   `json:"local_blobs"`
	RemoteBlobs        int   `json:"remote_blobs"`
	RetainedTemporary  int   `json:"retained_temporary"`
	RelayRetained      int   `json:"relay_retained"`
	Pinned             int   `json:"pinned"`
	Expired            int   `json:"expired"`
	Deleted            int   `json:"deleted"`
	Encrypted          int   `json:"encrypted"`
	AvailableForResend int   `json:"available_for_resend"`
	LocalBytes         int64 `json:"local_bytes"`
	RelayBytes         int64 `json:"relay_bytes"`
}

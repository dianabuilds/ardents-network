package model

import "time"

type Ref struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Object struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Owner     string         `json:"owner"`
	Body      map[string]any `json:"body,omitempty"`
	BlobRefs  []Ref          `json:"blob_refs,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Blob struct {
	ID        string    `json:"id"`
	CID       string    `json:"cid,omitempty"`
	MediaType string    `json:"media_type"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash,omitempty"`
	Cipher    string    `json:"cipher,omitempty"`
	KeyID     string    `json:"key_id,omitempty"`
	Nonce     string    `json:"nonce,omitempty"`
	State     string    `json:"state,omitempty"`
	Retention string    `json:"retention,omitempty"`
	Encrypted bool      `json:"encrypted"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Manifest struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Owner     string         `json:"owner"`
	Refs      []Ref          `json:"refs,omitempty"`
	Access    string         `json:"access,omitempty"`
	Retention string         `json:"retention,omitempty"`
	Encrypted bool           `json:"encrypted"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

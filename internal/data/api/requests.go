package api

import "time"

type PublishObjectCommand struct {
	Object ObjectSnapshot `json:"object"`
}

type GetObjectQuery struct {
	ID string `json:"id,omitempty"`
}

type ListObjectsQuery struct{}

type PublishBlobCommand struct {
	Blob BlobSnapshot `json:"blob"`
}

type FetchBlobCommand struct {
	ID string `json:"id,omitempty"`
}

type GetBlobQuery struct {
	ID string `json:"id,omitempty"`
}

type ListBlobsQuery struct{}

type PublishManifestCommand struct {
	Manifest ManifestSnapshot `json:"manifest"`
}

type GetManifestQuery struct {
	ID string `json:"id,omitempty"`
}

type ListManifestsQuery struct{}

type DataInventoryQuery struct{}

type ListBlobSourcesQuery struct {
	ID string `json:"id,omitempty"`
}

type GetTransferQuery struct {
	ID string `json:"id,omitempty"`
}

type ListTransfersQuery struct{}

type RetainBlobCommand struct {
	ID        string    `json:"id,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type PinBlobCommand struct {
	ID string `json:"id,omitempty"`
}

type DropBlobCommand struct {
	ID string `json:"id,omitempty"`
}

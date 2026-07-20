package state

import (
	"ardents/internal/data/availability"
	model "ardents/internal/data/model"
	"ardents/internal/data/placement"
)

type Snapshot struct {
	Objects      map[string]model.Object             `json:"objects"`
	Blobs        map[string]model.Blob               `json:"blobs"`
	Sources      map[string][]model.BlobSourceRecord `json:"sources"`
	Transfers    map[string]model.TransferRecord     `json:"transfers"`
	Manifests    map[string]model.Manifest           `json:"manifests"`
	Placement    placement.State                     `json:"placement"`
	Availability availability.State                  `json:"availability"`
}

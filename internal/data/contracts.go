package data

import (
	"time"

	dataapi "ardents/internal/data/api"
	availabilitypkg "ardents/internal/data/availability"
	model "ardents/internal/data/model"
)

type Ref = model.Ref
type Object = model.Object
type Blob = model.Blob
type SourceTrust = model.SourceTrust
type BlobSourceRecord = model.BlobSourceRecord
type TransferRecord = model.TransferRecord
type Manifest = model.Manifest
type Inventory = model.Inventory
type ReplicaIntent = availabilitypkg.ReplicaIntent
type RepairRecord = availabilitypkg.RepairRecord
type AvailabilitySnapshot = availabilitypkg.Snapshot
type AvailabilityReconcileResult = availabilitypkg.ReconcileResult

type BlobSource interface {
	GetBlob(string) (Blob, bool)
	GetBlobPayload(string) ([]byte, error)
}

type RetentionAuthorizer func(blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time) error

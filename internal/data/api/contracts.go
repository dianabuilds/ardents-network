package api

import (
	"context"
	"time"
)

type ObjectService interface {
	PublishObject(ObjectSnapshot) (ObjectSnapshot, error)
	GetObject(string) (ObjectSnapshot, error)
	ListObjects() ([]ObjectSnapshot, error)
}

type BlobService interface {
	PublishBlob(BlobSnapshot) (BlobSnapshot, error)
	GetBlob(string) (BlobSnapshot, error)
	ListBlobs() ([]BlobSnapshot, error)
}

type ManifestService interface {
	PublishManifest(ManifestSnapshot) (ManifestSnapshot, error)
	GetManifest(string) (ManifestSnapshot, error)
	ListManifests() ([]ManifestSnapshot, error)
}

type RetentionService interface {
	RetainBlob(string, time.Time) (BlobSnapshot, error)
	PinBlob(string) (BlobSnapshot, error)
	DropBlob(string) (BlobSnapshot, error)
}

type TransferService interface {
	FetchBlob(context.Context, string) (BlobSnapshot, error)
	GetTransfer(string) (TransferSnapshot, error)
	ListTransfers() []TransferSnapshot
}

type CatalogService interface {
	ObjectPart() PartSnapshot
	BlobPart() PartSnapshot
	DataInventory() DataInventorySnapshot
	ListBlobSources(string) []BlobSourceSnapshot
}

type AvailabilityService interface {
	SetReplicaIntent(ReplicaIntentSnapshot) (ReplicaIntentSnapshot, error)
	ReconcileDataAvailability(context.Context) error
	GetAvailability(string) (AvailabilitySnapshot, error)
	ListReplicaRepairs(string) []RepairSnapshot
}

type Service interface {
	ObjectService
	BlobService
	ManifestService
	RetentionService
	TransferService
	CatalogService
	AvailabilityService
}

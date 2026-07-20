package process

import (
	"context"
	"time"

	dataapi "ardents/internal/data/api"
	diagapi "ardents/internal/diagnostics/api"
	discoveryapi "ardents/internal/discovery/api"
	hostingapi "ardents/internal/hosting/api"
	nodeapi "ardents/internal/node/api"
	runtimeconfig "ardents/internal/runtime/config"
	workloadapi "ardents/internal/workload/api"
)

type RuntimeSurface interface {
	Start(context.Context) error
	Stop(context.Context) error
	Snapshot() nodeapi.Snapshot
	Subscribe(context.Context) <-chan nodeapi.Event
}

type NodeRuntime interface {
	RuntimeSurface
	GetNodeRuntime() nodeapi.NodeRuntimeSnapshot
	GetNetworkStatus() nodeapi.NetworkStatusSnapshot
	GetDiscoveryStatus() nodeapi.DiscoveryStatusSnapshot
	GetLocalPresence() nodeapi.LocalPresenceSnapshot
	ListPeers() []nodeapi.PeerSnapshot
	ListRouteCandidates(nodeapi.ListRouteCandidatesQuery) ([]discoveryapi.RouteCandidateSnapshot, discoveryapi.RouteSnapshot, error)
	Capabilities() nodeapi.CapabilitiesSnapshot
	DiagnosticsSnapshot() diagapi.DiagSnapshot
	PendingOperations() []diagapi.OperationSnapshot
	ListRecords() ([]discoveryapi.DiscoveryRecord, error)
	ResolveRecord(string, string) (discoveryapi.DiscoveryResult, error)
	ImportRecord(discoveryapi.DiscoveryRecord) (discoveryapi.RecordImportResult, error)
	ResolveService(string) (discoveryapi.ServiceResult, error)
	RoutingDetails() discoveryapi.RouteSnapshot
	ListWorkloads() ([]workloadapi.WorkloadStatusSnapshot, error)
	GetWorkloadStatus(string) (workloadapi.WorkloadStatusSnapshot, error)
	RegisterWorkload(workloadapi.WorkloadSpecSnapshot) error
	RegisterWorkloadContext(context.Context, workloadapi.WorkloadSpecSnapshot) error
	StartWorkload(string) error
	StartWorkloadContext(context.Context, string) error
	StopWorkload(string) error
	StopWorkloadContext(context.Context, string) error
	RestartWorkload(string) error
	RestartWorkloadContext(context.Context, string) error
	GetHostedService(string) (hostingapi.HostedServiceStatusSnapshot, error)
	ListHostedServices() ([]hostingapi.HostedServiceSnapshot, error)
	GetServicePublicationStatus(string) (hostingapi.PublicationStatusSnapshot, error)
	PublishBlob(dataapi.BlobSnapshot) (dataapi.BlobSnapshot, error)
	GetBlob(string) (dataapi.BlobSnapshot, error)
	ListBlobs() ([]dataapi.BlobSnapshot, error)
	PublishObject(dataapi.ObjectSnapshot) (dataapi.ObjectSnapshot, error)
	GetObject(string) (dataapi.ObjectSnapshot, error)
	ListObjects() ([]dataapi.ObjectSnapshot, error)
	PublishManifest(dataapi.ManifestSnapshot) (dataapi.ManifestSnapshot, error)
	GetManifest(string) (dataapi.ManifestSnapshot, error)
	ListManifests() ([]dataapi.ManifestSnapshot, error)
	FetchBlob(context.Context, string) (dataapi.BlobSnapshot, error)
	FetchChunked(context.Context, string) (dataapi.ChunkFetchSnapshot, error)
	ListBlobSources(string) []dataapi.BlobSourceSnapshot
	GetTransfer(string) (dataapi.TransferSnapshot, error)
	ListTransfers() []dataapi.TransferSnapshot
	PinBlob(string) (dataapi.BlobSnapshot, error)
	RetainBlob(string, time.Time) (dataapi.BlobSnapshot, error)
	DropBlob(string) (dataapi.BlobSnapshot, error)
	ObjectPart() dataapi.PartSnapshot
	BlobPart() dataapi.PartSnapshot
	DataInventory() dataapi.DataInventorySnapshot
	SetReplicaIntent(dataapi.ReplicaIntentSnapshot) (dataapi.ReplicaIntentSnapshot, error)
	ReconcileDataAvailability(context.Context) error
	GetAvailability(string) (dataapi.AvailabilitySnapshot, error)
	ListReplicaRepairs(string) []dataapi.RepairSnapshot
	GetEffectiveConfig() runtimeconfig.EffectiveSnapshot
	ReloadConfig(context.Context) runtimeconfig.ReloadResult
	GetHealthSummary() diagapi.HealthSnapshot
	ExplainFailure(string, string) diagapi.FailureExplanationSnapshot
	ListRecentEvents(int, string) ([]diagapi.EventEnvelope, string)
	RecordEventCommand(diagapi.RecordEventCommand) diagapi.EventEnvelope
	NodeForTesting() any
}

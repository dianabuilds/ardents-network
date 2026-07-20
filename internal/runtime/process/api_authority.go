package process

import (
	"context"
	"time"

	dataapi "ardents/internal/data/api"
	discoveryapi "ardents/internal/discovery/api"
	workloadapi "ardents/internal/workload/api"
)

func (n *Node) ResolveRecord(subject, kind string) (discoveryapi.DiscoveryResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ResolveRecordLocked(subject, kind)
}

func (n *Node) ResolveService(serviceType string) (discoveryapi.ServiceResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ResolveServiceLocked(serviceType)
}

func (n *Node) ListRecords() ([]discoveryapi.DiscoveryRecord, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ListRecordsLocked()
}

func (n *Node) ImportRecord(record discoveryapi.DiscoveryRecord) (discoveryapi.RecordImportResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ImportRecordLocked(record)
}

func (n *Node) ListWorkloads() ([]workloadapi.WorkloadStatusSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ListWorkloadsLocked()
}

func (n *Node) GetWorkloadStatus(id string) (workloadapi.WorkloadStatusSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.GetWorkloadLocked(id)
}

func (n *Node) RegisterWorkload(spec workloadapi.WorkloadSpecSnapshot) error {
	return n.RegisterWorkloadContext(context.Background(), spec)
}

func (n *Node) RegisterWorkloadContext(ctx context.Context, spec workloadapi.WorkloadSpecSnapshot) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.RegisterWorkloadLocked(ctx, spec)
}

func (n *Node) StartWorkload(id string) error {
	return n.StartWorkloadContext(context.Background(), id)
}

func (n *Node) StartWorkloadContext(ctx context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.StartWorkloadLocked(ctx, id)
}

func (n *Node) StopWorkload(id string) error {
	return n.StopWorkloadContext(context.Background(), id)
}

func (n *Node) StopWorkloadContext(ctx context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.StopWorkloadLocked(ctx, id)
}

func (n *Node) RestartWorkload(id string) error {
	return n.RestartWorkloadContext(context.Background(), id)
}

func (n *Node) RestartWorkloadContext(ctx context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.RestartWorkloadLocked(ctx, id)
}

func (n *Node) PublishObject(object dataapi.ObjectSnapshot) (dataapi.ObjectSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.PublishObjectLocked(object)
}

func (n *Node) GetObject(id string) (dataapi.ObjectSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.GetObjectLocked(id)
}

func (n *Node) ListObjects() ([]dataapi.ObjectSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ListObjectsLocked()
}

func (n *Node) PublishBlob(blob dataapi.BlobSnapshot) (dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.PublishBlobLocked(blob)
}

func (n *Node) FetchBlob(ctx context.Context, id string) (dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.FetchBlobLocked(ctx, id)
}

func (n *Node) FetchChunked(ctx context.Context, rootID string) (dataapi.ChunkFetchSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.FetchChunkedLocked(ctx, rootID)
}

func (n *Node) GetBlob(id string) (dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.GetBlobLocked(id)
}

func (n *Node) ListBlobs() ([]dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ListBlobsLocked()
}

func (n *Node) PublishManifest(manifest dataapi.ManifestSnapshot) (dataapi.ManifestSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.PublishManifestLocked(manifest)
}

func (n *Node) GetManifest(id string) (dataapi.ManifestSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.GetManifestLocked(id)
}

func (n *Node) ListManifests() ([]dataapi.ManifestSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ListManifestsLocked()
}

func (n *Node) RetainBlob(id string, expiresAt time.Time) (dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.RetainBlobLocked(id, expiresAt)
}

func (n *Node) PinBlob(id string) (dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.PinBlobLocked(id)
}

func (n *Node) DropBlob(id string) (dataapi.BlobSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.DropBlobLocked(id)
}

func (n *Node) DataInventory() dataapi.DataInventorySnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.DataInventoryLocked()
}

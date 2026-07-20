package process

import (
	"context"
	"errors"

	dataapi "ardents/internal/data/api"
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics/api"
	hostingapi "ardents/internal/hosting/api"
)

func (n *Node) ListBlobSources(id string) []dataapi.BlobSourceSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.ListBlobSourceSnapshots(id)
}

func (n *Node) GetTransfer(id string) (dataapi.TransferSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	item, ok := n.data.GetTransferSnapshot(id)
	if !ok {
		return dataapi.TransferSnapshot{}, errors.New("transfer not found")
	}
	return item, nil
}

func (n *Node) ListTransfers() []dataapi.TransferSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.ListTransferSnapshots()
}

func (n *Node) ObjectPart() dataapi.PartSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.ObjectPart()
}

func (n *Node) BlobPart() dataapi.PartSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.BlobPart()
}

func (n *Node) GetHealthSummary() diagapi.HealthSnapshot {
	_ = n.SyncDiagnostics(context.Background())
	return diagnostics.HealthSnapshot(n.DiagnosticsRecorder().Health())
}

func (n *Node) ExplainFailure(scope, resourceID string) diagapi.FailureExplanationSnapshot {
	_ = n.SyncDiagnostics(context.Background())
	health := n.DiagnosticsRecorder().Health()
	var service *hostingapi.HostedServiceStatusSnapshot
	if scope == "service" && resourceID != "" {
		status, err := n.GetHostedService(resourceID)
		if err == nil {
			service = &status
		}
	}
	return diagnostics.ExplainFailureSnapshot(scope, resourceID, health, service)
}

func (n *Node) ListRecentEvents(limit int, cursor string) ([]diagapi.EventEnvelope, string) {
	_ = n.SyncDiagnostics(context.Background())
	return diagnostics.RecentEventEnvelopes(n.DiagnosticsRecorder().Snapshot().RecentEvents, limit, cursor)
}

func (n *Node) RecordEventCommand(command diagapi.RecordEventCommand) diagapi.EventEnvelope {
	return n.DiagnosticsRecorder().RecordEventCommand(command)
}

func (n *Node) SyncDiagnostics(_ context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.SyncDiagnosticsLocked()
}

func (n *Node) DiagnosticsRecorder() *diagnostics.Recorder {
	return n.diag
}

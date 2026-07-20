package process

import (
	"context"

	dataapi "ardents/internal/data/api"
)

func (n *Node) SetReplicaIntent(intent dataapi.ReplicaIntentSnapshot) (dataapi.ReplicaIntentSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.SetReplicaIntentSnapshotLocked(intent)
}

func (n *Node) ReconcileDataAvailability(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ReconcileDataAvailabilityLocked(ctx)
}

func (n *Node) GetAvailability(rootManifestID string) (dataapi.AvailabilitySnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.GetDataAvailabilityLocked(rootManifestID)
}

func (n *Node) ListReplicaRepairs(rootManifestID string) []dataapi.RepairSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ListReplicaRepairsLocked(rootManifestID)
}

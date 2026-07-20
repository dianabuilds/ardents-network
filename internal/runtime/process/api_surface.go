package process

import (
	diagapi "ardents/internal/diagnostics/api"
	discoveryapi "ardents/internal/discovery/api"
	nodeapi "ardents/internal/node/api"
)

func (n *Node) Snapshot() nodeapi.Snapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n.queryService.SnapshotLocked()
}

func (n *Node) IdentitySnapshot() nodeapi.IdentitySnapshot { return n.Snapshot().Ident }
func (n *Node) TrustSnapshot() nodeapi.TrustSnapshot       { return n.Snapshot().Trust }
func (n *Node) DiscoverySnapshot() nodeapi.DiscoverySnapshot {
	return n.Snapshot().Disco
}
func (n *Node) TransportSnapshot() nodeapi.PartSnapshot { return n.Snapshot().Trans }
func (n *Node) RoutingPartSnapshot() nodeapi.PartSnapshot {
	return n.Snapshot().Route
}
func (n *Node) DataSnapshot() (nodeapi.PartSnapshot, nodeapi.PartSnapshot) {
	snap := n.Snapshot()
	return snap.Object, snap.Blob
}
func (n *Node) WorkloadStateSnapshot() nodeapi.WorkloadStateSnapshot { return n.Snapshot().Workload }

func (n *Node) RoutingDetails() discoveryapi.RouteSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.RoutingDetailsLocked()
}

func (n *Node) DiagnosticsSnapshot() diagapi.DiagSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n.queryService.DiagnosticsSnapshotLocked()
}

func (n *Node) PendingOperations() []diagapi.OperationSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.PendingOperationsLocked()
}

func (n *Node) Capabilities() nodeapi.CapabilitiesSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.CapabilitiesSnapshotLocked()
}

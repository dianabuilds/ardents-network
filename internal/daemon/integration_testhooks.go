//go:build integration

package daemon

import (
	"context"
	"errors"
	"time"

	appdata "ardents/internal/content"
	"ardents/internal/network"
	"ardents/internal/replication"
	"ardents/internal/replication/availability"
	"ardents/internal/workload/execution"
	domainworkload "ardents/internal/workload/registry"
)

func ReplaceWorkloadForIntegrationTest(n *Node, svc *execution.Service) {
	n.workload = svc
	n.workload.SetAdmission(func(spec domainworkload.Spec, items []execution.Status) error {
		return n.policy.AdmitWorkload(spec, items)
	})
	n.data.SetRetentionAuthorizer(func(blob appdata.BlobPolicyView, relay bool, expiresAt time.Time) error {
		return n.policy.AllowBlobRetention(blob, relay, expiresAt, time.Now().UTC())
	})
	n.trans.SetBootstrapObserver(n.handleBootstrapDialLocked)
	n.initOwnerCollaboratorsLocked()
	n.runtimeMgr.SyncObservedTruthLocked()
}

func SetBlobExchangeStarterForIntegrationTest(n *Node, fn func(context.Context) error) {
	n.startBlobExchange = fn
}

func PlaceBlobReplicaForIntegrationTest(n *Node, ctx context.Context, blobID, target string, intentVersion uint64) (replication.ReplicaCommitment, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data place replica"); err != nil {
		return replication.ReplicaCommitment{}, err
	}
	return n.remoteData.PlaceBlob(ctx, blobID, target, intentVersion)
}

func PlaceAvailableBlobReplicasForIntegrationTest(n *Node, ctx context.Context, blobID string, count int, intentVersion uint64) (replication.ReplicaPlacementOutcome, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data place available replicas"); err != nil {
		return replication.ReplicaPlacementOutcome{}, err
	}
	return n.remoteData.PlaceAvailable(ctx, blobID, count, intentVersion)
}

func ProbeBlobReplicaForIntegrationTest(n *Node, ctx context.Context, commitment replication.ReplicaCommitment) (replication.ReplicaCommitment, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data probe replica"); err != nil {
		return replication.ReplicaCommitment{}, err
	}
	return n.remoteData.ProbeReplica(ctx, commitment)
}

func RecordRepairFailureForIntegrationTest(n *Node, repairID string, at time.Time) (availability.RepairRecord, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.replica.RecordRepairFailure(repairID, at, "insufficient capacity")
}

func ReplicaPlacementStateForIntegrationTest(n *Node) replication.ReplicaPlacementSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return replicaPlacementSnapshot(n.replica.ReplicaPlacementState())
}

func TransportStateForIntegrationTest(n *Node) string {
	return n.trans.State()
}

func TransportHealthSignalsForIntegrationTest(n *Node) network.HealthSignals {
	return n.trans.HealthSignals()
}

func NetworkSideEffectsClearedForIntegrationTest(n *Node) bool {
	return n.cancel == nil && n.network == nil
}

func StopTransportForIntegrationTest(n *Node, ctx context.Context) error {
	return n.trans.Stop(ctx)
}

func SetReachabilityForIntegrationTest(n *Node, state string) error {
	target, ok := n.trans.(interface{ SetReachabilityForIntegration(string) error })
	if !ok {
		return errors.New("transport reachability integration hook unavailable")
	}
	return target.SetReachabilityForIntegration(state)
}

func WorkloadStatusForIntegrationTest(n *Node, id string) (execution.Status, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.workload.Get(id)
}

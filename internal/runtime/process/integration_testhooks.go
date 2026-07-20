//go:build integration

package process

import (
	"context"
	"errors"
	"time"

	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
	dataplacement "ardents/internal/data/placement"
	datareplication "ardents/internal/data/replication"
	runtimeassembly "ardents/internal/runtime/assembly"
	workloadcontroller "ardents/internal/workload/controller"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

func ReplaceWorkloadForIntegrationTest(n *Node, svc *workloadcontroller.Service) {
	n.workload = svc
	n.workload.SetAdmission(func(spec domainworkload.Spec, items []observedstate.Status) error {
		return n.policy.AdmitWorkload(spec, items)
	})
	n.data.SetRetentionAuthorizer(func(blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time) error {
		return n.policy.AllowBlobRetention(blob, relay, expiresAt, time.Now().UTC())
	})
	n.trans.SetBootstrapObserver(n.handleBootstrapDialLocked)
	collaborators := runtimeassembly.New(runtimeAssemblyConfig(n))
	n.publicationMgr = collaborators.Publication
	n.authorityCtl = collaborators.Authority
	n.runtimeMgr = collaborators.Runtime
	n.queryService = collaborators.Query
	n.commandService = collaborators.Command
	n.runtimeMgr.SyncObservedTruthLocked()
}

func SetBlobExchangeStarterForIntegrationTest(n *Node, fn func(context.Context) error) {
	n.startBlobExchange = fn
}

func PlaceBlobReplicaForIntegrationTest(n *Node, ctx context.Context, blobID, target string, intentVersion uint64) (dataplacement.Commitment, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.PlaceBlobReplicaLocked(ctx, blobID, target, intentVersion)
}

func PlaceAvailableBlobReplicasForIntegrationTest(n *Node, ctx context.Context, blobID string, count int, intentVersion uint64) (datareplication.PlacementOutcome, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.PlaceAvailableBlobReplicasLocked(ctx, blobID, count, intentVersion)
}

func ProbeBlobReplicaForIntegrationTest(n *Node, ctx context.Context, commitment dataplacement.Commitment) (dataplacement.Commitment, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ProbeBlobReplicaLocked(ctx, commitment)
}

func SetReplicaIntentForIntegrationTest(n *Node, intent appdata.ReplicaIntent) (appdata.ReplicaIntent, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.SetReplicaIntentLocked(intent)
}

func ReconcileDataAvailabilityForIntegrationTest(n *Node, ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.ReconcileDataAvailabilityLocked(ctx)
}

func DataAvailabilityForIntegrationTest(n *Node, rootManifestID string) (appdata.AvailabilitySnapshot, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.authorityCtl.DataAvailabilityLocked(rootManifestID)
}

func RecordRepairFailureForIntegrationTest(n *Node, repairID string, at time.Time) (appdata.RepairRecord, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.RecordRepairFailure(repairID, at, "insufficient capacity")
}

func ReplicaPlacementStateForIntegrationTest(n *Node) dataplacement.State {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.ReplicaPlacementState()
}

func TransportStateForIntegrationTest(n *Node) string {
	return n.trans.State()
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

func WorkloadStatusForIntegrationTest(n *Node, id string) (observedstate.Status, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.workload.Get(id)
}

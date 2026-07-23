package daemon

import (
	"ardents/internal/content"
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"ardents/internal/identity"
	networkprivacy "ardents/internal/messaging"
	"ardents/internal/network"
	noderoute "ardents/internal/network/routing"
	apppolicy "ardents/internal/policy"
	"ardents/internal/workload"
	"ardents/internal/workload/execution"
	"context"
	"time"
)

type PartSnapshot struct {
	State  string
	Reason string
}

type NodeSnapshot struct {
	Name      string
	State     string
	Ready     bool
	Reason    string
	Lifecycle diagnostics.LifecycleSnapshot
}

type BootSnapshot struct {
	Joined bool
	State  string
	Reason string
	Source []string
}

type RuntimeSnapshot struct {
	Node     NodeSnapshot
	Boot     BootSnapshot
	Identity identity.Snapshot
	Health   diagnostics.HealthSnapshot
}

type SystemSnapshot struct {
	Node      NodeSnapshot
	Boot      BootSnapshot
	Ident     identity.Snapshot
	Trust     discovery.TrustSnapshot
	Disco     discovery.SummarySnapshot
	Trans     PartSnapshot
	Transport *network.Snapshot
	Route     PartSnapshot
	Object    PartSnapshot
	Blob      PartSnapshot
	Policy    PartSnapshot
	Workload  workload.StateSnapshot
	Store     content.StoreSnapshot
	Diag      diagnostics.DiagSnapshot
}

func projectRuntime(snapshot SystemSnapshot, health diagnostics.HealthSnapshot) RuntimeSnapshot {
	return RuntimeSnapshot{Node: snapshot.Node, Boot: snapshot.Boot, Identity: snapshot.Ident, Health: health}
}

type NodeFeatures struct {
	Version  string
	Services []string
	Features map[string]bool
}

type TrustExplanation struct {
	Outcome string
	Reason  string
	Valid   bool
	Trusted bool
	Usable  bool
}

type dataProjectionReader interface {
	ObjectPart() content.PartSnapshot
	BlobPart() content.PartSnapshot
	InventorySnapshot() content.InventorySnapshot
}

type runtimeSync interface {
	SyncObservedTruthLocked()
}

type workloadSync interface {
	SyncObserved(context.Context) error
}

type QueryService struct {
	name        string
	nodeProfile network.NodeProfile
	boot        *BootStatus
	life        *diagnostics.Machine
	diag        *diagnostics.Recorder
	ident       identity.Service
	trust       *discovery.TrustEvaluator
	disco       *discovery.Service
	trans       network.Service
	privacy     *networkprivacy.Channel
	dataPrivacy *networkprivacy.Channel
	route       *noderoute.State
	policy      apppolicy.Policy
	data        dataProjectionReader
	workload    *execution.Service
	runtime     runtimeSync
	observed    workloadSync
}

func newQueryService(
	name string,
	nodeProfile network.NodeProfile,
	boot *BootStatus,
	life *diagnostics.Machine,
	diag *diagnostics.Recorder,
	ident identity.Service,
	trustSvc *discovery.TrustEvaluator,
	disco *discovery.Service,
	trans network.Service,
	privacy *networkprivacy.Channel,
	dataPrivacy *networkprivacy.Channel,
	route *noderoute.State,
	policySvc apppolicy.Policy,
	dataSvc dataProjectionReader,
	workloadSvc *execution.Service,
) *QueryService {
	return &QueryService{
		name:        name,
		nodeProfile: nodeProfile,
		boot:        boot,
		life:        life,
		diag:        diag,
		ident:       ident,
		trust:       trustSvc,
		disco:       disco,
		trans:       trans,
		privacy:     privacy,
		dataPrivacy: dataPrivacy,
		route:       route,
		policy:      policySvc,
		data:        dataSvc,
		workload:    workloadSvc,
	}
}

func (r *QueryService) bindSynchronizers(runtime runtimeSync, workload workloadSync) {
	r.runtime = runtime
	r.observed = workload
}

func (r *QueryService) projectSnapshotLocked() SystemSnapshot {
	return SystemSnapshot{
		Node:      r.nodeSnapshotLocked(),
		Boot:      r.bootSnapshotLocked(),
		Ident:     r.identitySnapshotLocked(),
		Trust:     r.trustSnapshotLocked(),
		Disco:     r.discoverySnapshotLocked(),
		Trans:     r.transportSnapshotLocked(),
		Transport: r.transportProfileSnapshotLocked(),
		Route:     r.routeSnapshotLocked(),
		Object:    r.objectPartSnapshotLocked(),
		Blob:      r.blobPartSnapshotLocked(),
		Policy:    r.policySnapshotLocked(),
		Workload:  r.workloadSnapshotLocked(),
		Store:     r.storeSnapshotLocked(),
		Diag:      diagnostics.ProjectSnapshot(r.diag.Snapshot()),
	}
}

func (r *QueryService) projectDiagnosticsLocked() diagnostics.DiagSnapshot {
	return diagnostics.ProjectSnapshot(r.diag.Snapshot())
}

func (r *QueryService) RoutingDetailsLocked() discovery.RouteSnapshot {
	return discovery.ProjectRoute(r.route.Last())
}

func (r *QueryService) PendingOperationsLocked() []diagnostics.OperationSnapshot {
	return diagnostics.ProjectOperations(r.diag.PendingOperations())
}

func (r *QueryService) RecentDiagnosticsLocked(limit int) []string {
	return r.diag.Last(limit)
}

func (r *QueryService) NodeFeatures() NodeFeatures {
	return NodeFeatures{
		Version: "v1alpha",
		Services: []string{
			"node",
			"identity",
			"discovery",
			"trust",
			"transport",
			"data",
			"workload",
			"diagnostics",
		},
		Features: map[string]bool{
			"node_start_stop":       true,
			"node_events":           true,
			"identity_snapshot":     true,
			"identity_mutations":    false,
			"discovery_resolution":  true,
			"discovery_publish":     false,
			"trust_evaluation":      true,
			"transport_snapshots":   true,
			"routing_candidates":    true,
			"data_objects":          true,
			"data_blobs":            true,
			"workload_control":      true,
			"hosted_service_status": true,
			"diagnostics_snapshots": true,
			"pending_operations":    true,
			"typed_rpc_contract":    true,
		},
	}
}

func (r *QueryService) EvaluateTrustLocked(record discovery.CatalogRecordSnapshot) discovery.TrustSnapshot {
	result := r.trust.Evaluate(discovery.RecordFromSnapshot(record))
	return discovery.ProjectTrust(discovery.TrustStateForResult(result), result)
}

func (r *QueryService) ExplainTrustLocked(record discovery.CatalogRecordSnapshot) TrustExplanation {
	result := r.trust.Evaluate(discovery.RecordFromSnapshot(record))
	return TrustExplanation{
		Outcome: result.Outcome,
		Reason:  result.Reason,
		Valid:   result.Valid,
		Trusted: result.Trusted,
		Usable:  result.Usable,
	}
}

func (r *QueryService) LastTransportCandidatesLocked() []network.Candidate {
	if route := r.route.Last(); route.Selected != nil {
		return []network.Candidate{*route.Selected}
	}
	return nil
}

func (r *QueryService) reasonOrDefaultLocked() string {
	if health := r.diag.Health(); health.PrimaryReason != nil {
		return health.PrimaryReason.Summary
	}
	return ""
}

func (r *QueryService) nodeSnapshotLocked() NodeSnapshot {
	life := r.life.Snapshot()
	nodeState := life.Current
	if nodeState == "" {
		nodeState = r.life.State()
	}
	return NodeSnapshot{
		Name:      r.name,
		State:     nodeState,
		Ready:     nodeState == diagnostics.Ready,
		Reason:    r.reasonOrDefaultLocked(),
		Lifecycle: diagnostics.APISnapshot(life),
	}
}

func (r *QueryService) bootSnapshotLocked() BootSnapshot {
	return BootSnapshot{
		Joined: r.bootJoinedLocked(),
		State:  r.bootStateLocked(),
		Reason: r.bootReasonLocked(),
		Source: cloneStrings(r.bootSourcesLocked()),
	}
}

func (r *QueryService) identitySnapshotLocked() identity.Snapshot {
	return identity.ProjectSnapshot(r.ident)
}

func (r *QueryService) trustSnapshotLocked() discovery.TrustSnapshot {
	result, state := r.observedTrustSnapshotLocked()
	return discovery.ProjectTrust(state, result)
}

func (r *QueryService) discoverySnapshotLocked() discovery.SummarySnapshot {
	id := r.ident.NodeSummary()
	return discovery.SummarySnapshot{
		State:     r.disco.State(),
		Reason:    r.disco.Reason(),
		Records:   r.disco.Count(""),
		LocalNode: id.Principal,
		Services:  r.disco.Count("service"),
	}
}

func (r *QueryService) transportSnapshotLocked() PartSnapshot {
	return partSnapshot(r.trans.State(), r.trans.Reason())
}

func (r *QueryService) transportProfileSnapshotLocked() *network.Snapshot {
	return new(r.trans.ProfileSnapshot())
}

func (r *QueryService) routeSnapshotLocked() PartSnapshot {
	return partSnapshot(r.route.State(), r.route.Reason())
}

func (r *QueryService) objectPartSnapshotLocked() PartSnapshot {
	part := r.data.ObjectPart()
	return partSnapshot(part.State, part.Reason)
}

func (r *QueryService) blobPartSnapshotLocked() PartSnapshot {
	part := r.data.BlobPart()
	return partSnapshot(part.State, part.Reason)
}

func (r *QueryService) policySnapshotLocked() PartSnapshot {
	snapshot := r.policy.Snapshot()
	return partSnapshot(snapshot.State, snapshot.Reason)
}

func (r *QueryService) workloadSnapshotLocked() workload.StateSnapshot {
	return workload.StateSnapshot{
		State:   r.workload.State(),
		Desired: r.workload.Desired(),
		Active:  r.workload.Active(),
	}
}

func (r *QueryService) storeSnapshotLocked() content.StoreSnapshot {
	inventory := r.data.InventorySnapshot()
	return content.ProjectStore(inventory, r.ident.NodeSummary().Principal != "")
}

func (r *QueryService) bootJoinedLocked() bool {
	return r.boot.Result().Joined
}

func (r *QueryService) bootStateLocked() string {
	return r.boot.Result().State
}

func (r *QueryService) bootReasonLocked() string {
	return r.boot.Result().Reason
}

func (r *QueryService) bootSourcesLocked() []string {
	return r.boot.Sources()
}

func partSnapshot(state, reason string) PartSnapshot {
	return PartSnapshot{State: state, Reason: reason}
}

func (r *QueryService) NodeRuntimeSnapshotLocked() RuntimeSnapshot {
	r.syncObservedTruthLocked()
	return projectRuntime(r.projectSnapshotLocked(), diagnostics.ProjectHealth(r.diag.Health()))
}

func (r *QueryService) NetworkStatusSnapshotLocked() network.StatusSnapshot {
	r.runtime.SyncObservedTruthLocked()
	profile := r.trans.ProfileSnapshot()
	return network.ProjectStatus(
		r.nodeProfile,
		r.trans.State(),
		r.trans.Reason(),
		r.boot.Result().Joined,
		profile,
		r.trans.ReachabilitySnapshot(),
		r.trans.AbuseSnapshot(),
		r.life.Snapshot().TransitionedAt,
		privateMessagingStatus(networkprivacy.Snapshot(r.privacy, r.dataPrivacy)),
	)
}

func privateMessagingStatus(status networkprivacy.StatusSnapshot) network.PrivateMessagingStatus {
	return network.PrivateMessagingStatus{
		Profile: status.Profile, State: status.State, SwitchReason: status.SwitchReason,
		RecoveryState: status.RecoveryState, ReducedFeatures: status.ReducedFeatures,
		ErrorCategories: status.ErrorCategories,
	}
}

func (r *QueryService) DiscoveryStatusSnapshotLocked(now time.Time) discovery.StatusSnapshot {
	r.runtime.SyncObservedTruthLocked()
	return discovery.ProjectStatus(
		r.disco.State(),
		r.disco.Reason(),
		r.disco.Entries(),
		now,
		r.trust.Evaluate,
	)
}

func (r *QueryService) PeerSnapshotsLocked() []discovery.PeerSnapshot {
	r.runtime.SyncObservedTruthLocked()
	return discovery.ProjectPeers(
		r.disco.Entries(),
		r.ident.NodeSummary().Principal,
		func(record discovery.Record, trusted bool) (string, string) {
			return network.PeerReachability(r.trans.BuildCandidates(network.RouteRecord{
				Subject: record.Subject(), Service: record.ServiceType(), Mode: record.ServiceMode(), Endpoints: record.EndpointList(),
			}, trusted))
		},
		r.trust.Evaluate,
	)
}

func (r *QueryService) SyncDiagnosticsLocked() error {
	r.runtime.SyncObservedTruthLocked()
	return r.observed.SyncObserved(context.Background())
}

func (r *QueryService) SnapshotLocked() SystemSnapshot {
	r.syncObservedTruthLocked()
	return r.projectSnapshotLocked()
}

func (r *QueryService) DiagnosticsSnapshotLocked() diagnostics.DiagSnapshot {
	r.syncObservedTruthLocked()
	return r.projectDiagnosticsLocked()
}

func (r *QueryService) NodeFeaturesSnapshotLocked() NodeFeaturesSnapshot {
	features := r.NodeFeatures()
	return NodeFeaturesSnapshot{
		Version:  features.Version,
		Services: cloneStrings(features.Services),
		Features: cloneBoolMap(features.Features),
	}
}

func (r *QueryService) syncObservedTruthLocked() {
	if r.runtime != nil {
		r.runtime.SyncObservedTruthLocked()
	}
	if r.observed != nil {
		if err := r.observed.SyncObserved(context.Background()); err != nil {
			r.diag.RecordEvent("workload", "observed_sync_failed", r.name, "workload observed state refresh failed", "workload.observed_sync_failed", map[string]any{"error": err.Error()})
		}
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (r *QueryService) observedTrustSnapshotLocked() (discovery.TrustResult, string) {
	localID := r.ident.NodeSummary().Principal
	var (
		localResult   discovery.TrustResult
		localFound    bool
		degradedTrust discovery.TrustResult
		degradedFound bool
	)
	for _, entry := range r.disco.Entries() {
		result := r.trust.Evaluate(entry.Record)
		if entry.Source == "local" && entry.Record.Kind() == "node" && entry.Record.Subject() == localID {
			localResult = result
			localFound = true
		}
		if result.Usable || trustResultIsAdvisory(result) {
			continue
		}
		if !degradedFound || trustResultSeverity(result) > trustResultSeverity(degradedTrust) {
			degradedTrust = result
			degradedFound = true
		}
	}
	switch {
	case degradedFound:
		return degradedTrust, discovery.TrustStateForResult(degradedTrust)
	case localFound:
		return localResult, discovery.TrustStateForResult(localResult)
	default:
		last := r.trust.Last()
		return last, r.trust.State()
	}
}

func trustResultIsAdvisory(result discovery.TrustResult) bool {
	return result.Valid && !result.Trusted
}

func trustResultSeverity(result discovery.TrustResult) int {
	switch {
	case result.Outcome == "expired":
		return 2
	case !result.Valid:
		return 2
	default:
		return 1
	}
}

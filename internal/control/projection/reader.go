package projection

import (
	dataapi "ardents/internal/data/api"
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics/api"
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	discoverystate "ardents/internal/discovery/state"
	hostingregistry "ardents/internal/hosting/registry"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	noderoute "ardents/internal/network/route"
	nodeapi "ardents/internal/node/api"
	nodelifecycle "ardents/internal/node/lifecycle"
	noderecovery "ardents/internal/node/recovery"
	policyapi "ardents/internal/policy/api"
	workloadcontroller "ardents/internal/workload/controller"
)

type Capabilities struct {
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
	ObjectPart() dataapi.PartSnapshot
	BlobPart() dataapi.PartSnapshot
	DataInventory() dataapi.DataInventorySnapshot
}

type Reader struct {
	name        string
	nodeProfile transport.NodeProfile
	boot        *noderecovery.BootStatus
	life        *nodelifecycle.Machine
	diag        *diagnostics.Recorder
	ident       identityapi.Service
	trust       *discovery.TrustEvaluator
	disco       *discovery.Service
	trans       transport.Service
	privacy     *networkprivacy.Channel
	dataPrivacy *networkprivacy.Channel
	route       *noderoute.State
	policy      policyapi.Service
	data        dataProjectionReader
	workload    *workloadcontroller.Service
	srv         *hostingregistry.Registry
}

func NewReader(
	name string,
	nodeProfile transport.NodeProfile,
	boot *noderecovery.BootStatus,
	life *nodelifecycle.Machine,
	diag *diagnostics.Recorder,
	ident identityapi.Service,
	trustSvc *discovery.TrustEvaluator,
	disco *discovery.Service,
	trans transport.Service,
	privacy *networkprivacy.Channel,
	dataPrivacy *networkprivacy.Channel,
	route *noderoute.State,
	policySvc policyapi.Service,
	dataSvc dataProjectionReader,
	workloadSvc *workloadcontroller.Service,
	srv *hostingregistry.Registry,
) *Reader {
	return &Reader{
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
		srv:         srv,
	}
}

func (r *Reader) SnapshotLocked() nodeapi.Snapshot {
	return nodeapi.Snapshot{
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
		Diag:      diagnostics.DiagnosticsSnapshot(r.diag.Snapshot()),
	}
}

func (r *Reader) DiagnosticsSnapshotLocked() diagapi.DiagSnapshot {
	return diagnostics.DiagnosticsSnapshot(r.diag.Snapshot())
}

func (r *Reader) RoutingDetailsLocked() discoveryapi.RouteSnapshot {
	return Route(r.route.Last())
}

func (r *Reader) PendingOperationsLocked() []diagapi.OperationSnapshot {
	return diagnostics.OperationSnapshots(r.diag.PendingOperations())
}

func (r *Reader) RecentDiagnosticsLocked(limit int) []string {
	return r.diag.Last(limit)
}

func (r *Reader) Capabilities() Capabilities {
	return Capabilities{
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

func (r *Reader) EvaluateTrustLocked(record discoveryapi.DiscoveryRecord) discoveryapi.TrustSnapshot {
	result := r.trust.Evaluate(DiscoveryRecord(record))
	return discoverystate.TrustSnapshot(discovery.TrustStateForResult(result), result)
}

func (r *Reader) ExplainTrustLocked(record discoveryapi.DiscoveryRecord) TrustExplanation {
	result := r.trust.Evaluate(DiscoveryRecord(record))
	return TrustExplanation{
		Outcome: result.Outcome,
		Reason:  result.Reason,
		Valid:   result.Valid,
		Trusted: result.Trusted,
		Usable:  result.Usable,
	}
}

func (r *Reader) TrustAnchorsLocked() []string {
	return r.trust.Anchors()
}

func (r *Reader) LastTransportCandidatesLocked() []transport.Candidate {
	if route := r.route.Last(); route.Selected != nil {
		return []transport.Candidate{*route.Selected}
	}
	return nil
}

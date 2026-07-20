package projection

import (
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	discoverystate "ardents/internal/discovery/state"
	transport "ardents/internal/network/api"
	noderoute "ardents/internal/network/route"
	nodeapi "ardents/internal/node/api"
	nodelifecycle "ardents/internal/node/lifecycle"
)

func (r *Reader) reasonOrDefaultLocked() string {
	if health := r.diag.Health(); health.PrimaryReason != nil {
		return health.PrimaryReason.Summary
	}
	return ""
}

func (r *Reader) nodeSnapshotLocked() nodeapi.NodeSnapshot {
	life := r.life.Snapshot()
	nodeState := life.Current
	if nodeState == "" {
		nodeState = r.life.State()
	}
	return nodeapi.NodeSnapshot{
		Name:      r.name,
		State:     nodeState,
		Ready:     nodeState == nodelifecycle.Ready,
		Reason:    r.reasonOrDefaultLocked(),
		Lifecycle: nodelifecycle.APISnapshot(life),
	}
}

func (r *Reader) bootSnapshotLocked() nodeapi.BootSnapshot {
	return nodeapi.BootSnapshot{
		Joined: r.bootJoinedLocked(),
		State:  r.bootStateLocked(),
		Reason: r.bootReasonLocked(),
		Source: cloneStrings(r.bootSourcesLocked()),
	}
}

func (r *Reader) identitySnapshotLocked() nodeapi.IdentitySnapshot {
	id := r.ident.NodeSummary()
	return nodeapi.IdentitySnapshot{
		State:     r.ident.NodeState(),
		Principal: id.Principal,
		Device:    id.Device,
		PublicKey: id.PublicKey,
		Source:    r.ident.NodeSource(),
	}
}

func (r *Reader) trustSnapshotLocked() nodeapi.TrustSnapshot {
	result, state := r.observedTrustSnapshotLocked()
	return nodeapi.TrustSnapshot{
		State:   state,
		Outcome: result.Outcome,
		Reason:  result.Reason,
		Valid:   result.Valid,
		Trusted: result.Trusted,
		Usable:  result.Usable,
	}
}

func (r *Reader) discoverySnapshotLocked() nodeapi.DiscoverySnapshot {
	id := r.ident.NodeSummary()
	return nodeapi.DiscoverySnapshot{
		State:     r.disco.State(),
		Reason:    r.disco.Reason(),
		Records:   r.disco.Count(""),
		LocalNode: id.Principal,
		Services:  r.disco.Count("service"),
	}
}

func (r *Reader) transportSnapshotLocked() nodeapi.PartSnapshot {
	return partSnapshot(r.trans.State(), r.trans.Reason())
}

func (r *Reader) transportProfileSnapshotLocked() *nodeapi.TransportSnapshot {
	snapshot := r.trans.ProfileSnapshot()
	return &nodeapi.TransportSnapshot{
		Profile:             string(snapshot.Profile),
		Mode:                string(snapshot.Mode),
		Health:              string(snapshot.Health),
		ActiveFamilies:      transportFamilies(snapshot.ActiveFamilies),
		SuppressedFamilies:  transportFamilies(snapshot.SuppressedFamilies),
		SwitchReason:        string(snapshot.SwitchReason),
		SwitchAutomatic:     snapshot.SwitchAutomatic,
		ReducedCapabilities: cloneStrings(snapshot.ReducedCapabilities),
		ActiveCapabilities:  cloneStrings(snapshot.ActiveCapabilities),
		RecoveryState:       string(snapshot.RecoveryState),
	}
}

func (r *Reader) routeSnapshotLocked() nodeapi.PartSnapshot {
	return partSnapshot(r.route.State(), r.route.Reason())
}

func (r *Reader) objectPartSnapshotLocked() nodeapi.PartSnapshot {
	part := r.data.ObjectPart()
	return partSnapshot(part.State, part.Reason)
}

func (r *Reader) blobPartSnapshotLocked() nodeapi.PartSnapshot {
	part := r.data.BlobPart()
	return partSnapshot(part.State, part.Reason)
}

func (r *Reader) policySnapshotLocked() nodeapi.PartSnapshot {
	return r.policy.Snapshot()
}

func (r *Reader) workloadSnapshotLocked() nodeapi.WorkloadStateSnapshot {
	return nodeapi.WorkloadStateSnapshot{
		State:   r.workload.State(),
		Desired: r.workload.Desired(),
		Active:  r.workload.Active(),
	}
}

func (r *Reader) storeSnapshotLocked() nodeapi.StoreSnapshot {
	inventory := r.data.DataInventory()
	authority := 0
	if r.ident.NodeSummary().Principal != "" {
		authority = 1
	}
	return nodeapi.StoreSnapshot{
		Authority: authority,
		Cached:    inventory.RemoteBlobs,
		Derived:   inventory.Objects + inventory.Manifests,
		Pinned:    inventory.Pinned,
	}
}

func (r *Reader) bootJoinedLocked() bool {
	return r.boot.Result().Joined
}

func (r *Reader) bootStateLocked() string {
	return r.boot.Result().State
}

func (r *Reader) bootReasonLocked() string {
	return r.boot.Result().Reason
}

func (r *Reader) bootSourcesLocked() []string {
	return r.boot.Sources()
}

func DiscoveryRecord(in discoveryapi.DiscoveryRecord) discovery.Record {
	return discovery.Record{
		ID:        in.ID,
		Kind:      in.Kind,
		Subject:   in.Subject,
		Node:      in.Node,
		Device:    in.Device,
		Owner:     in.Owner,
		Service:   in.Service,
		Mode:      in.Mode,
		PublicKey: in.PublicKey,
		Endpoints: cloneStrings(in.Endpoints),
		IssuedAt:  in.IssuedAt,
		ExpiresAt: in.ExpiresAt,
		Signature: in.Signature,
	}
}

func Record(entry discovery.Entry) discoveryapi.DiscoveryRecord {
	return discoverystate.RecordSnapshot(entry)
}

func TrustSnapshot(state string, result discovery.TrustResult) nodeapi.TrustSnapshot {
	trust := discoverystate.TrustSnapshot(state, result)
	return nodeapi.TrustSnapshot{
		State:   trust.State,
		Outcome: trust.Outcome,
		Reason:  trust.Reason,
		Valid:   trust.Valid,
		Trusted: trust.Trusted,
		Usable:  trust.Usable,
	}
}

func Targets(items []transport.Candidate) []discoveryapi.TransportTarget {
	if len(items) == 0 {
		return nil
	}
	out := make([]discoveryapi.TransportTarget, 0, len(items))
	for _, item := range items {
		out = append(out, discoveryapi.TransportTarget{
			Subject:     item.Subject,
			Service:     item.Service,
			Endpoint:    item.Endpoint,
			Scheme:      item.Scheme,
			Mode:        item.Mode,
			Trusted:     item.Trusted,
			Usable:      item.Usable,
			Cost:        item.Cost,
			Privacy:     item.Privacy,
			Reliability: item.Reliability,
		})
	}
	return out
}

func transportFamilies(items []transport.TransportFamily) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}

func Route(in noderoute.Snapshot) discoveryapi.RouteSnapshot {
	out := discoveryapi.RouteSnapshot{
		Outcome:    in.Outcome,
		Reason:     in.Reason,
		Candidates: in.Candidates,
		Usable:     in.Usable,
	}
	if in.Selected != nil {
		selected := Targets([]transport.Candidate{*in.Selected})
		if len(selected) > 0 {
			out.Selected = &selected[0]
		}
	}
	return out
}

func partSnapshot(state, reason string) nodeapi.PartSnapshot {
	return nodeapi.PartSnapshot{State: state, Reason: reason}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}

func CloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func SplitTopic(topic string) (string, string) {
	for i := 0; i < len(topic); i++ {
		if topic[i] == '.' {
			return topic[:i], topic[i+1:]
		}
	}
	return "node", topic
}

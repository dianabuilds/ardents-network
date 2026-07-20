package process

import (
	"errors"
	"time"

	discoveryapi "ardents/internal/discovery/api"
	nodeapi "ardents/internal/node/api"
)

func (n *Node) GetNodeRuntime() nodeapi.NodeRuntimeSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.NodeRuntimeSnapshotLocked()
}

func (n *Node) GetNetworkStatus() nodeapi.NetworkStatusSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.NetworkStatusSnapshotLocked()
}

func (n *Node) GetDiscoveryStatus() nodeapi.DiscoveryStatusSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.DiscoveryStatusSnapshotLocked(time.Now().UTC())
}

func (n *Node) GetLocalPresence() nodeapi.LocalPresenceSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n.publicationMgr.LocalPresenceSnapshotLocked()
}

func (n *Node) ListPeers() []nodeapi.PeerSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.PeerSnapshotsLocked()
}

func (n *Node) ListRouteCandidates(query nodeapi.ListRouteCandidatesQuery) ([]discoveryapi.RouteCandidateSnapshot, discoveryapi.RouteSnapshot, error) {
	switch {
	case query.Service != "":
		result, err := n.ResolveService(query.Service)
		if err != nil {
			return nil, discoveryapi.RouteSnapshot{}, err
		}
		items := make([]discoveryapi.TransportTarget, 0)
		for _, match := range result.Matches {
			items = append(items, match.Candidates...)
		}
		return routeCandidateSnapshots(items), result.Route, nil
	case query.Subject != "" || query.Kind != "":
		result, err := n.ResolveRecord(query.Subject, query.Kind)
		if err != nil {
			return nil, discoveryapi.RouteSnapshot{}, err
		}
		return routeCandidateSnapshots(result.Candidates), result.Route, nil
	default:
		return nil, discoveryapi.RouteSnapshot{}, errors.New("route candidate query requires subject/kind or service")
	}
}

func routeCandidateSnapshots(items []discoveryapi.TransportTarget) []discoveryapi.RouteCandidateSnapshot {
	out := make([]discoveryapi.RouteCandidateSnapshot, 0, len(items))
	for _, item := range items {
		state := "candidate"
		reason := ""
		if !item.Usable {
			state = "degraded"
			reason = "route candidate is not usable"
		}
		out = append(out, discoveryapi.RouteCandidateSnapshot{
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
			State:       state,
			Reason:      reason,
		})
	}
	return out
}

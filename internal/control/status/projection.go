package status

import (
	"time"

	diagapi "ardents/internal/diagnostics/api"
	discovery "ardents/internal/discovery"
	discoverystate "ardents/internal/discovery/state"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	nodeapi "ardents/internal/node/api"
	nodelifecycle "ardents/internal/node/lifecycle"
)

func NodeRuntimeSnapshot(snapshot nodeapi.Snapshot, health diagapi.HealthSnapshot) nodeapi.NodeRuntimeSnapshot {
	return nodeapi.NodeRuntimeSnapshot{
		Node:     snapshot.Node,
		Boot:     snapshot.Boot,
		Identity: snapshot.Ident,
		Health:   health,
	}
}

func NetworkStatusSnapshot(
	nodeProfile transport.NodeProfile,
	state string,
	reason string,
	joined bool,
	profile transport.Snapshot,
	reachability transport.ReachabilitySnapshot,
	abuse transport.AbuseSnapshot,
	lastTransitionAt time.Time,
	privacy networkprivacy.StatusSnapshot,
) nodeapi.NetworkStatusSnapshot {
	return nodeapi.NetworkStatusSnapshot{
		NodeProfile:             string(nodeProfile),
		State:                   state,
		Reason:                  reason,
		Joined:                  joined,
		Reachable:               reachability.Reachable,
		ReachabilityMode:        string(reachability.Mode),
		ReachabilityState:       reachability.State,
		ReachabilityReason:      reachability.Reason,
		ReachabilityObservedAt:  reachability.ObservedAt,
		ActiveProfile:           string(profile.Profile),
		ActiveMode:              string(profile.Mode),
		ReducedCapabilities:     append(cloneStrings(profile.ReducedCapabilities), privacy.ReducedCapabilities...),
		ActiveCapabilities:      cloneStrings(profile.ActiveCapabilities),
		AbuseState:              abuse.State,
		AbuseReason:             abuse.Reason,
		RateLimitedOperations:   abuse.RateLimitedOperations,
		BackpressuredOperations: abuse.BackpressuredOperations,
		OversizedMessages:       abuse.OversizedMessages,
		BannedProviders:         abuse.BannedProviders,
		LastTransitionAt:        lastTransitionAt,
		PrivacyProfile:          privacy.Profile,
		PrivacyState:            privacy.State,
		PrivacySwitchReason:     privacy.SwitchReason,
		PrivacyRecoveryState:    privacy.RecoveryState,
		PrivacyErrors:           cloneStrings(privacy.ErrorCategories),
	}
}

func DiscoveryStatusSnapshot(
	state string,
	reason string,
	entries []discovery.Entry,
	now time.Time,
	evaluate func(discovery.Record) discovery.TrustResult,
) nodeapi.DiscoveryStatusSnapshot {
	var status nodeapi.DiscoveryStatusSnapshot
	status.State = state
	status.Reason = reason
	for _, item := range entries {
		if item.Source == "local" {
			status.LocalRecords++
			if item.SeenAt.After(status.LastPublishAt) {
				status.LastPublishAt = item.SeenAt
			}
		} else {
			status.RemoteRecords++
			if item.SeenAt.After(status.LastRefreshAt) {
				status.LastRefreshAt = item.SeenAt
			}
		}
		if !item.Record.ExpiresAt.IsZero() && now.After(item.Record.ExpiresAt) {
			status.StaleRecords++
			status.RejectedRecords++
			continue
		}
		trust := evaluate(item.Record)
		if trust.Trusted && trust.Usable {
			status.TrustedRecords++
			continue
		}
		if !trust.Usable || !trust.Valid || trust.Outcome == "expired" {
			status.RejectedRecords++
		}
	}
	return status
}

func PeerSnapshots(
	entries []discovery.Entry,
	localID string,
	buildCandidates func(discovery.Record, bool) []transport.Candidate,
	evaluate func(discovery.Record) discovery.TrustResult,
) []nodeapi.PeerSnapshot {
	out := make([]nodeapi.PeerSnapshot, 0, len(entries))
	for _, item := range entries {
		snapshot, ok := peerSnapshot(item, localID, buildCandidates, evaluate)
		if ok {
			out = append(out, snapshot)
		}
	}
	return out
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}

func peerSnapshot(
	item discovery.Entry,
	localID string,
	buildCandidates func(discovery.Record, bool) []transport.Candidate,
	evaluate func(discovery.Record) discovery.TrustResult,
) (nodeapi.PeerSnapshot, bool) {
	if item.Record.Kind != "node" || item.Record.Subject == localID {
		return nodeapi.PeerSnapshot{}, false
	}
	trust := evaluate(item.Record)
	candidates := buildCandidates(item.Record, trust.Trusted)
	reachability, reason := peerReachability(candidates)
	state := peerState(trust, reachability)
	if reason == "" {
		reason = trust.Reason
	}
	return nodeapi.PeerSnapshot{
		NodeID:       item.Record.Node,
		DeviceID:     item.Record.Device,
		Addresses:    cloneStrings(item.Record.Endpoints),
		Trust:        trustSnapshot(trust),
		Reachability: reachability,
		Source:       item.Source,
		LastSeenAt:   item.SeenAt,
		State:        state,
		Reason:       reason,
	}, true
}

func peerReachability(items []transport.Candidate) (string, string) {
	if len(items) == 0 {
		return "unreachable", "peer has no advertised endpoints"
	}
	for _, item := range items {
		if item.Usable {
			return "reachable", ""
		}
	}
	return "limited", "peer endpoints are not currently usable"
}

func NodeReady(state string) bool {
	return state == nodelifecycle.Ready
}

func peerState(trust discovery.TrustResult, reachability string) string {
	state := "ready"
	if !trust.Usable || reachability != "reachable" {
		state = "degraded"
	}
	if !trust.Valid {
		state = "failed"
	}
	return state
}

func trustSnapshot(trust discovery.TrustResult) nodeapi.TrustSnapshot {
	snapshot := discoverystate.TrustSnapshot(discovery.TrustStateForResult(trust), trust)
	return nodeapi.TrustSnapshot{
		State:   snapshot.State,
		Outcome: snapshot.Outcome,
		Reason:  snapshot.Reason,
		Valid:   snapshot.Valid,
		Trusted: snapshot.Trusted,
		Usable:  snapshot.Usable,
	}
}

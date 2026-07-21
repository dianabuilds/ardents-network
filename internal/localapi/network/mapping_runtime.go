package network

import (
	"ardents/internal/discovery"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
	domain "ardents/internal/network"
)

func operationStatus(state, reason string, accepted bool) *protocol.OperationStatus {
	return &protocol.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}

func networkStatus(in domain.StatusSnapshot) *protocol.NetworkStatusSnapshot {
	return &protocol.NetworkStatusSnapshot{
		State: in.State, Reason: in.Reason, Joined: in.Joined, Reachable: in.Reachable,
		ActiveProfile: in.ActiveProfile, ActiveMode: in.ActiveMode,
		ReducedCapabilities: append([]string(nil), in.ReducedCapabilities...), LastTransitionAt: rpc.Timestamp(in.LastTransitionAt),
		PrivacyProfile: in.PrivacyProfile, PrivacyState: in.PrivacyState,
		PrivacySwitchReason: in.PrivacySwitchReason, PrivacyRecoveryState: in.PrivacyRecoveryState,
		PrivacyErrorCategories: append([]string(nil), in.PrivacyErrors...), NodeProfile: in.NodeProfile,
		ReachabilityMode: in.ReachabilityMode, ReachabilityState: in.ReachabilityState,
		ReachabilityReason: in.ReachabilityReason, ReachabilityObservedAt: rpc.Timestamp(in.ReachabilityObservedAt),
		ActiveCapabilities: append([]string(nil), in.ActiveCapabilities...), AbuseState: in.AbuseState,
		AbuseReason: in.AbuseReason, RateLimitedOperations: in.RateLimitedOperations,
		BackpressuredOperations: in.BackpressuredOperations, OversizedMessages: in.OversizedMessages,
		BannedProviders: int32(in.BannedProviders),
	}
}

func toPeerSnapshots(items []discovery.PeerSnapshot) []*protocol.PeerSnapshot {
	out := make([]*protocol.PeerSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &protocol.PeerSnapshot{NodeId: item.NodeID, DeviceId: item.DeviceID,
			Addresses: append([]string(nil), item.Addresses...), Trust: toDiscoveryTrustSnapshot(item.Trust),
			Reachability: item.Reachability, Source: item.Source, LastSeenAt: rpc.Timestamp(item.LastSeenAt),
			State: item.State, Reason: item.Reason})
	}
	return out
}

package connectrpc

import (
	nodeapi "ardents/internal/node/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toNodeRuntimeSnapshot(in nodeapi.NodeRuntimeSnapshot) *ardentsv1.NodeRuntimeSnapshot {
	return &ardentsv1.NodeRuntimeSnapshot{
		Node:     toNodeSnapshot(in.Node),
		Boot:     toBootSnapshot(in.Boot),
		Identity: toIdentitySnapshot(in.Identity),
		Health:   toHealthSnapshot(in.Health),
	}
}

func toNetworkStatusSnapshot(in nodeapi.NetworkStatusSnapshot) *ardentsv1.NetworkStatusSnapshot {
	return &ardentsv1.NetworkStatusSnapshot{
		State:                   in.State,
		Reason:                  in.Reason,
		Joined:                  in.Joined,
		Reachable:               in.Reachable,
		ActiveProfile:           in.ActiveProfile,
		ActiveMode:              in.ActiveMode,
		ReducedCapabilities:     append([]string(nil), in.ReducedCapabilities...),
		LastTransitionAt:        ts(in.LastTransitionAt),
		PrivacyProfile:          in.PrivacyProfile,
		PrivacyState:            in.PrivacyState,
		PrivacySwitchReason:     in.PrivacySwitchReason,
		PrivacyRecoveryState:    in.PrivacyRecoveryState,
		PrivacyErrorCategories:  append([]string(nil), in.PrivacyErrors...),
		NodeProfile:             in.NodeProfile,
		ReachabilityMode:        in.ReachabilityMode,
		ReachabilityState:       in.ReachabilityState,
		ReachabilityReason:      in.ReachabilityReason,
		ReachabilityObservedAt:  ts(in.ReachabilityObservedAt),
		ActiveCapabilities:      append([]string(nil), in.ActiveCapabilities...),
		AbuseState:              in.AbuseState,
		AbuseReason:             in.AbuseReason,
		RateLimitedOperations:   in.RateLimitedOperations,
		BackpressuredOperations: in.BackpressuredOperations,
		OversizedMessages:       in.OversizedMessages,
		BannedProviders:         int32(in.BannedProviders),
	}
}

func toPeerSnapshots(items []nodeapi.PeerSnapshot) []*ardentsv1.PeerSnapshot {
	out := make([]*ardentsv1.PeerSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &ardentsv1.PeerSnapshot{
			NodeId:       item.NodeID,
			DeviceId:     item.DeviceID,
			Addresses:    append([]string(nil), item.Addresses...),
			Trust:        toTrustSnapshot(item.Trust),
			Reachability: item.Reachability,
			Source:       item.Source,
			LastSeenAt:   ts(item.LastSeenAt),
			State:        item.State,
			Reason:       item.Reason,
		})
	}
	return out
}

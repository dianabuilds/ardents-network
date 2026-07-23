package node

import (
	"ardents/internal/content"
	daemonruntime "ardents/internal/daemon"
	"ardents/internal/discovery"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/network"
	"maps"
)

func statusProto(state, reason string, accepted bool) *ardentsv1.OperationStatus {
	return &ardentsv1.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}

func toNodeFeaturesSnapshot(in daemonruntime.NodeFeaturesSnapshot) *ardentsv1.NodeFeaturesSnapshot {
	if in.Version == "" && len(in.Services) == 0 && len(in.Features) == 0 {
		return nil
	}
	return &ardentsv1.NodeFeaturesSnapshot{Version: in.Version, Services: append([]string(nil), in.Services...), Features: cloneBoolMap(in.Features)}
}

func toPartSnapshot(in daemonruntime.PartSnapshot) *ardentsv1.PartSnapshot {
	return &ardentsv1.PartSnapshot{State: in.State, Reason: in.Reason}
}

func toTrustSnapshot(in discovery.TrustSnapshot) *ardentsv1.TrustSnapshot {
	return &ardentsv1.TrustSnapshot{State: in.State, Outcome: in.Outcome, Reason: in.Reason, Valid: in.Valid, Trusted: in.Trusted, Usable: in.Usable}
}

func toDiscoverySnapshot(in discovery.SummarySnapshot) *ardentsv1.DiscoverySnapshot {
	return &ardentsv1.DiscoverySnapshot{State: in.State, Reason: in.Reason, Records: int32(in.Records), LocalNode: in.LocalNode, Services: int32(in.Services)}
}

func toTransportSnapshot(in *network.Snapshot) (*ardentsv1.TransportSnapshot, error) {
	if in == nil {
		return nil, nil
	}
	reduced, err := network.TransportFeatureStrings(in.ReducedFeatures)
	if err != nil {
		return nil, err
	}
	active, err := network.TransportFeatureStrings(in.ActiveFeatures)
	if err != nil {
		return nil, err
	}
	return &ardentsv1.TransportSnapshot{
		Profile:            string(in.Profile),
		Mode:               string(in.Mode),
		Health:             string(in.Health),
		ActiveFamilies:     networkFamilies(in.ActiveFamilies),
		SuppressedFamilies: networkFamilies(in.SuppressedFamilies),
		SwitchReason:       string(in.SwitchReason),
		SwitchAutomatic:    in.SwitchAutomatic,
		ReducedFeatures:    reduced,
		ActiveFeatures:     active,
		RecoveryState:      string(in.RecoveryState),
	}, nil
}

func networkFamilies(items []network.Family) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	maps.Copy(out, in)
	return out
}

func toStoreSnapshot(in content.StoreSnapshot) *ardentsv1.StoreSnapshot {
	return &ardentsv1.StoreSnapshot{Authority: int32(in.Authority), Cached: int32(in.Cached), Derived: int32(in.Derived), Pinned: int32(in.Pinned)}
}

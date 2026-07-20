package connectrpc

import (
	nodeapi "ardents/internal/node/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func statusProto(state, reason string, accepted bool) *ardentsv1.OperationStatus {
	return &ardentsv1.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}

func toProtoError(in rpcError) *ardentsv1.Error {
	return &ardentsv1.Error{
		Code:      in.Code,
		Category:  in.Category,
		Message:   in.Message,
		Domain:    in.Domain,
		Retryable: in.Retryable,
		Operation: in.Operation,
		Reason:    in.Reason,
		Details:   toStruct(in.Details),
	}
}

func toCapabilitiesSnapshot(in nodeapi.CapabilitiesSnapshot) *ardentsv1.CapabilitiesSnapshot {
	if in.Version == "" && len(in.Services) == 0 && len(in.Features) == 0 {
		return nil
	}
	return &ardentsv1.CapabilitiesSnapshot{Version: in.Version, Services: append([]string(nil), in.Services...), Features: cloneBoolMap(in.Features)}
}

func toPartSnapshot(in nodeapi.PartSnapshot) *ardentsv1.PartSnapshot {
	return &ardentsv1.PartSnapshot{State: in.State, Reason: in.Reason}
}

func toTrustSnapshot(in nodeapi.TrustSnapshot) *ardentsv1.TrustSnapshot {
	return &ardentsv1.TrustSnapshot{State: in.State, Outcome: in.Outcome, Reason: in.Reason, Valid: in.Valid, Trusted: in.Trusted, Usable: in.Usable}
}

func toDiscoverySnapshot(in nodeapi.DiscoverySnapshot) *ardentsv1.DiscoverySnapshot {
	return &ardentsv1.DiscoverySnapshot{State: in.State, Reason: in.Reason, Records: int32(in.Records), LocalNode: in.LocalNode, Services: int32(in.Services)}
}

func toTransportSnapshot(in *nodeapi.TransportSnapshot) *ardentsv1.TransportSnapshot {
	if in == nil {
		return nil
	}
	return &ardentsv1.TransportSnapshot{
		Profile:             in.Profile,
		Mode:                in.Mode,
		Health:              in.Health,
		ActiveFamilies:      append([]string(nil), in.ActiveFamilies...),
		SuppressedFamilies:  append([]string(nil), in.SuppressedFamilies...),
		SwitchReason:        in.SwitchReason,
		SwitchAutomatic:     in.SwitchAutomatic,
		ReducedCapabilities: append([]string(nil), in.ReducedCapabilities...),
		ActiveCapabilities:  append([]string(nil), in.ActiveCapabilities...),
		RecoveryState:       in.RecoveryState,
	}
}

func toStoreSnapshot(in nodeapi.StoreSnapshot) *ardentsv1.StoreSnapshot {
	return &ardentsv1.StoreSnapshot{Authority: int32(in.Authority), Cached: int32(in.Cached), Derived: int32(in.Derived), Pinned: int32(in.Pinned)}
}

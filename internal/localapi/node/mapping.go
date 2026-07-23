package node

import (
	daemonruntime "ardents/internal/daemon"
	"ardents/internal/identity"
	diagnosticsapi "ardents/internal/localapi/diagnostics"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/workload"
)

func toSnapshot(in daemonruntime.SystemSnapshot) *ardentsv1.Snapshot {
	return &ardentsv1.Snapshot{
		Node:      toNodeSnapshot(in.Node),
		Boot:      toBootSnapshot(in.Boot),
		Ident:     toIdentitySnapshot(in.Ident),
		Trust:     toTrustSnapshot(in.Trust),
		Disco:     toDiscoverySnapshot(in.Disco),
		Trans:     toPartSnapshot(in.Trans),
		Transport: toTransportSnapshot(in.Transport),
		Route:     toPartSnapshot(in.Route),
		Object:    toPartSnapshot(in.Object),
		Blob:      toPartSnapshot(in.Blob),
		Policy:    toPartSnapshot(in.Policy),
		Workload:  toWorkloadStateSnapshot(in.Workload),
		Store:     toStoreSnapshot(in.Store),
		Diag:      diagnosticsapi.DiagSnapshot(in.Diag),
	}
}

func toNodeSnapshot(in daemonruntime.NodeSnapshot) *ardentsv1.NodeSnapshot {
	return &ardentsv1.NodeSnapshot{Name: in.Name, State: in.State, Ready: in.Ready, Reason: in.Reason}
}

func toBootSnapshot(in daemonruntime.BootSnapshot) *ardentsv1.BootSnapshot {
	return &ardentsv1.BootSnapshot{Joined: in.Joined, State: in.State, Reason: in.Reason, Source: append([]string(nil), in.Source...)}
}

func toIdentitySnapshot(in identity.Snapshot) *ardentsv1.IdentitySnapshot {
	return &ardentsv1.IdentitySnapshot{State: in.State, Principal: in.Principal, PublicKey: in.PublicKey}
}

func toWorkloadStateSnapshot(in workload.StateSnapshot) *ardentsv1.WorkloadStateSnapshot {
	return &ardentsv1.WorkloadStateSnapshot{State: in.State, Desired: int32(in.Desired), Active: int32(in.Active)}
}

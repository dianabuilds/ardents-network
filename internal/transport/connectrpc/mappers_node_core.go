package connectrpc

import (
	nodeapi "ardents/internal/node/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toSnapshot(in nodeapi.Snapshot) *ardentsv1.Snapshot {
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
		Diag:      toDiagSnapshot(in.Diag),
	}
}

func toNodeSnapshot(in nodeapi.NodeSnapshot) *ardentsv1.NodeSnapshot {
	return &ardentsv1.NodeSnapshot{Name: in.Name, State: in.State, Ready: in.Ready, Reason: in.Reason}
}

func toBootSnapshot(in nodeapi.BootSnapshot) *ardentsv1.BootSnapshot {
	return &ardentsv1.BootSnapshot{Joined: in.Joined, State: in.State, Reason: in.Reason, Source: append([]string(nil), in.Source...)}
}

func toIdentitySnapshot(in nodeapi.IdentitySnapshot) *ardentsv1.IdentitySnapshot {
	return &ardentsv1.IdentitySnapshot{State: in.State, Principal: in.Principal, Device: in.Device, PublicKey: in.PublicKey}
}

func toWorkloadStateSnapshot(in nodeapi.WorkloadStateSnapshot) *ardentsv1.WorkloadStateSnapshot {
	return &ardentsv1.WorkloadStateSnapshot{State: in.State, Desired: int32(in.Desired), Active: int32(in.Active)}
}

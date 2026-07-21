package node

import (
	daemonruntime "ardents/internal/daemon"
	diagnosticsapi "ardents/internal/localapi/diagnostics"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func toNodeRuntimeSnapshot(in daemonruntime.RuntimeSnapshot) *ardentsv1.NodeRuntimeSnapshot {
	return &ardentsv1.NodeRuntimeSnapshot{
		Node:     toNodeSnapshot(in.Node),
		Boot:     toBootSnapshot(in.Boot),
		Identity: toIdentitySnapshot(in.Identity),
		Health:   diagnosticsapi.HealthSnapshot(in.Health),
	}
}

package node

import (
	daemonruntime "ardents/internal/daemon"
	diagnosticsapi "ardents/internal/localapi/diagnostics"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func toNodeRuntimeSnapshot(in daemonruntime.RuntimeSnapshot) *ardentsv1.NodeRuntimeSnapshot {
	return &ardentsv1.NodeRuntimeSnapshot{
		Node:      toNodeSnapshot(in.Node),
		Boot:      toBootSnapshot(in.Boot),
		Identity:  toIdentitySnapshot(in.Identity),
		Health:    diagnosticsapi.HealthSnapshot(in.Health),
		Readiness: toReadinessSnapshot(in.Readiness),
	}
}

func toReadinessSnapshot(in daemonruntime.ReadinessSnapshot) *ardentsv1.ReadinessSnapshot {
	checks := make([]*ardentsv1.ReadinessCheckSnapshot, 0, len(in.Checks))
	for _, check := range in.Checks {
		checks = append(checks, &ardentsv1.ReadinessCheckSnapshot{
			Name: check.Name, Ready: check.Ready, Reason: check.Reason,
		})
	}
	return &ardentsv1.ReadinessSnapshot{Ready: in.Ready, Reason: in.Reason, Checks: checks}
}

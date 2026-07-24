package daemon

import (
	"testing"

	"ardents/internal/diagnostics"
	"ardents/internal/identity"
	"ardents/internal/network"

	"github.com/stretchr/testify/require"
)

func TestEvaluateRolloutReadinessDegradationMatrix(t *testing.T) {
	healthyIdentity := identity.Snapshot{
		State:     "ready",
		Principal: "p1_node",
		PublicKey: "node-public-key",
	}
	healthyNetwork := network.StatusSnapshot{State: "ready"}
	healthyDiagnostics := diagnostics.HealthSnapshot{State: "ready"}

	tests := []struct {
		name        string
		identity    identity.Snapshot
		network     network.StatusSnapshot
		diagnostics diagnostics.HealthSnapshot
		ready       bool
		reason      string
	}{
		{
			name: "all retained runtime dependencies are ready", identity: healthyIdentity,
			network: healthyNetwork, diagnostics: healthyDiagnostics, ready: true,
		},
		{
			name: "network degraded", identity: healthyIdentity,
			network:     network.StatusSnapshot{State: "degraded", Reason: "relay unavailable"},
			diagnostics: healthyDiagnostics, reason: "network: relay unavailable",
		},
		{
			name: "diagnostics degraded", identity: healthyIdentity, network: healthyNetwork,
			diagnostics: diagnostics.HealthSnapshot{
				State: "degraded",
				PrimaryReason: &diagnostics.ReasonSnapshot{
					Code: "diagnostics.persistence.degraded", Summary: "diagnostics persistence degraded",
				},
			},
			reason: "diagnostics: diagnostics.persistence.degraded: diagnostics persistence degraded",
		},
		{
			name: "identity state degraded",
			identity: identity.Snapshot{
				State: "degraded", Principal: healthyIdentity.Principal, PublicKey: healthyIdentity.PublicKey,
			},
			network: healthyNetwork, diagnostics: healthyDiagnostics,
			reason: "identity: state is degraded",
		},
		{
			name:     "identity principal lost",
			identity: identity.Snapshot{State: "ready", PublicKey: healthyIdentity.PublicKey},
			network:  healthyNetwork, diagnostics: healthyDiagnostics,
			reason: "identity: retained Principal is missing",
		},
		{
			name:     "identity public key lost",
			identity: identity.Snapshot{State: "ready", Principal: healthyIdentity.Principal},
			network:  healthyNetwork, diagnostics: healthyDiagnostics,
			reason: "identity: retained public key is missing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := EvaluateRolloutReadiness(test.identity, test.network, test.diagnostics)

			require.Equal(t, test.ready, got.Ready)
			require.Equal(t, test.reason, got.Reason)
			require.Equal(t, []string{"network", "diagnostics", "identity"}, readinessCheckNames(got.Checks))
			for _, check := range got.Checks {
				if check.Name == "network" && test.network.State != "ready" ||
					check.Name == "diagnostics" && test.diagnostics.State != "ready" ||
					check.Name == "identity" && (test.identity.State != "ready" || test.identity.Principal == "" || test.identity.PublicKey == "") {
					require.False(t, check.Ready)
				}
			}
		})
	}
}

func readinessCheckNames(checks []ReadinessCheckSnapshot) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		out = append(out, check.Name)
	}
	return out
}

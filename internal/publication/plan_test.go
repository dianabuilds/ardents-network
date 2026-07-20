package publication

import (
	"testing"

	hostingreadiness "ardents/internal/hosting/readiness"
	hostingregistry "ardents/internal/hosting/registry"
	hostingservice "ardents/internal/hosting/service"
	networkreadiness "ardents/internal/network/readiness"

	"github.com/stretchr/testify/require"
)

func TestPublicationPlanSeparatesAllowedAndDeniedServices(t *testing.T) {
	ready := hostingreadiness.Snapshot{State: hostingreadiness.StateReady, Ready: true, ExposureEligible: true}
	allowed, denied := publicationGatePlan([]hostingregistry.ServiceStatus{
		{Spec: hostingservice.Spec{ID: "svc.allowed", Type: "echo", Owner: "work.a", Mode: "NetworkPublished",
			Endpoints: []string{"http://10.0.0.2:9000/ready"}, ProbeEndpoints: []string{"http://127.0.0.1:9000/ready"}}, Readiness: ready},
		{Spec: hostingservice.Spec{ID: "svc.denied", Type: "admin", Owner: "work.a", Mode: "NetworkPublished",
			Endpoints: []string{"http://10.0.0.2:9001/ready"}, ProbeEndpoints: []string{"http://127.0.0.1:9001/ready"}}, Readiness: ready},
		{Spec: hostingservice.Spec{ID: "svc.inactive", Type: "debug", Owner: "work.a", Mode: "NetworkPublished",
			Endpoints: []string{"http://10.0.0.2:9002/ready"}, ProbeEndpoints: []string{"http://127.0.0.1:9002/ready"}},
			Readiness: hostingreadiness.Snapshot{State: hostingreadiness.StateNotReady, Reason: hostingreadiness.ReasonListenerUnreachable}},
		{Spec: hostingservice.Spec{ID: "svc.not-exposure-eligible", Type: "debug", Owner: "work.a", Mode: "NetworkPublished",
			Endpoints: []string{"http://10.0.0.2:9003/ready"}, ProbeEndpoints: []string{"http://127.0.0.1:9003/ready"}},
			Readiness: hostingreadiness.Snapshot{State: hostingreadiness.StateReady, Ready: true, ExposureEligible: false}},
		{Spec: hostingservice.Spec{ID: "svc.local", Type: "local", Mode: "LocalOnly", Endpoints: []string{"unix:///tmp/local.sock"}}, Readiness: ready},
	}, networkreadiness.ReachabilitySnapshot{Mode: networkreadiness.ReachabilityPrivateLAN, State: "lan", Reachable: true}, func(spec hostingservice.Spec) error {
		if spec.ID == "svc.denied" {
			return assertErr("policy_publication_denied: service type is denied by policy")
		}
		return nil
	})
	require.Falsef(t, len(allowed) != 1 || allowed[0].ID != "svc.allowed", "allowed services = %#v, want only svc.allowed", allowed)
	require.Len(t, denied, 3)
	require.Equal(t, "svc.denied", denied[0].ID)
	require.Equal(t, "svc.inactive", denied[1].ID)
	require.Equal(t, "svc.not-exposure-eligible", denied[2].ID)
}

func TestPublicationPlanRejectsNetworkCapabilityLoss(t *testing.T) {
	ready := hostingreadiness.Snapshot{State: hostingreadiness.StateReady, Ready: true, ExposureEligible: true}
	items := []hostingregistry.ServiceStatus{{Spec: hostingservice.Spec{ID: "svc.net", Type: "echo", Mode: "NetworkPublished",
		Endpoints: []string{"http://10.0.0.2:9000/ready"}, ProbeEndpoints: []string{"http://127.0.0.1:9000/ready"}}, Readiness: ready}}
	allowed, denied := publicationGatePlan(items,
		networkreadiness.ReachabilitySnapshot{Mode: networkreadiness.ReachabilityOutboundOnly, State: "outbound_only"}, nil)
	require.Empty(t, allowed)
	require.Len(t, denied, 1)
	require.ErrorContains(t, denied[0].Err, "network")
}

func TestPublicationPlanRequiresEndpointScopeToMatchReachabilityMode(t *testing.T) {
	ready := hostingreadiness.Snapshot{State: hostingreadiness.StateReady, Ready: true, ExposureEligible: true}
	tests := []struct {
		name     string
		mode     networkreadiness.ReachabilityMode
		endpoint string
	}{
		{name: "public mode private address", mode: networkreadiness.ReachabilityPublicDirect, endpoint: "tcp://10.0.0.2:9000"},
		{name: "LAN mode public address", mode: networkreadiness.ReachabilityPrivateLAN, endpoint: "tcp://203.0.113.8:9000"},
		{name: "unverified DNS", mode: networkreadiness.ReachabilityPublicDirect, endpoint: "tcp://node.example:9000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := []hostingregistry.ServiceStatus{{Spec: hostingservice.Spec{ID: "svc", Type: "echo", Mode: "NetworkPublished",
				Endpoints: []string{test.endpoint}, ProbeEndpoints: []string{"tcp://127.0.0.1:9000"}}, Readiness: ready}}
			allowed, denied := publicationGatePlan(items,
				networkreadiness.ReachabilitySnapshot{Mode: test.mode, State: "reachable", Reachable: true}, nil)
			require.Empty(t, allowed)
			require.Len(t, denied, 1)
			require.ErrorContains(t, denied[0].Err, "scope")
		})
	}
}

func TestStaticHostedServiceStatusHasNoRuntimeBackedPublication(t *testing.T) {
	status := staticHostedServiceStatus(hostingservice.Spec{
		ID:        "svc.static",
		Type:      "echo",
		Owner:     "manual",
		Mode:      "LocalOnly",
		Endpoints: []string{"unix:///tmp/echo.sock"},
	})
	require.Equal(t, "svc.static", status.ID)
	require.False(t, status.Published)
	require.Equal(t, "static service has no runtime-backed publication", status.Reason)
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

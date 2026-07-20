//go:build integration

package networkfoundation_test

import (
	"testing"

	transport "ardents/internal/network/api"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestTransportBootstrapStatusDegradesAfterPeerLoss(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "network"},
		Speed:       "default",
		Environment: "local",
	})

	var remote transport.Service
	var local transport.Service

	scenario.Precondition("start bootstrapped transport pair", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		local = testkit.StartBootstrappedTransport(t, remote)
	})

	scenario.Step("wait for ready bootstrap status", func(t *testing.T) {
		testkit.WaitForRelayReadiness(t, local)
		status := testkit.WaitForBootstrapReady(t, local)
		require.True(t, status.Joined)
		require.Equal(t, "ready", status.State)
	})

	scenario.Degraded("peer loss degrades bootstrap status", func(t *testing.T) {
		require.NoError(t, remote.Stop(t.Context()))
		testkit.WaitForTransportDegradedAfterPeerLoss(t, local)
	})
}

func TestTransportBootstrapDialFailureIsReported(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "network"},
		Speed:       "default",
		Environment: "local",
	})

	remote := testkit.StartTransport(t)
	peers := append([]string(nil), remote.Endpoints()...)
	require.NoError(t, remote.Stop(t.Context()))

	local := transport.New()
	reports := make(chan transport.BootstrapDialReport, 4)

	scenario.Precondition("start local transport with unreachable bootstrap peers", func(t *testing.T) {
		testkit.ConfigureLoopbackTransport(t)
		local.SetBootstrapNodes(peers)
		local.SetBootstrapObserver(func(report transport.BootstrapDialReport) {
			reports <- report
		})
		require.NoError(t, local.Start(t.Context()))
		t.Cleanup(func() {
			_ = local.Stop(t.Context())
		})
	})

	scenario.Assert("bootstrap dial failure is reported and reflected in status", func(t *testing.T) {
		select {
		case report := <-reports:
			require.False(t, report.Success)
			require.NotEmpty(t, report.Peer)
			require.NotEmpty(t, report.Detail)
		case <-t.Context().Done():
			require.FailNow(t, "timed out waiting for bootstrap dial failure report")
		}

		status := local.BootstrapStatus()
		require.False(t, status.Joined)
		require.Equal(t, "degraded", status.State)
		require.NotEmpty(t, status.Reason)
	})
}

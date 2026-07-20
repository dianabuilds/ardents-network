//go:build e2e

package networkfoundatione2e_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"
	runtimeinfra "ardents/internal/runtime/process"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeTransportParticipationAndPeerLossVisibility(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-foundation",
		ScenarioID:  "NFE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "network"},
		Speed:       "default",
		Environment: "local",
	})

	var remote transport.Service
	var rt *testkit.RuntimeHarness

	scenario.Precondition("start remote transport and bootstrapped node", func(t *testing.T) {
		remote = testkit.StartTransport(t)

		rt = testkit.NewRuntime(t, runtimeinfra.Config{
			Name: "network-foundation-e2e",
			Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), remote.Endpoints()...)},
			Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
		})
		require.NoError(t, rt.Runtime.Start(t.Context()))
		t.Cleanup(func() {
			_ = rt.Runtime.Stop(t.Context())
		})
	})

	scenario.Step("node joins relay bootstrap peer", func(t *testing.T) {
		testkit.WaitForNodeBootstrapReady(t, rt.Runtime)
	})

	scenario.Degraded("peer loss is visible through node degradation", func(t *testing.T) {
		require.NoError(t, remote.Stop(t.Context()))
		testkit.WaitForNodeDegradedAfterPeerLoss(t, rt.Runtime)
	})
}

func TestNodeTransportRecoversAfterBootstrapPeerRestart(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-foundation",
		ScenarioID:  "NFE-002",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "network", "recovery"},
		Speed:       "default",
		Environment: "local",
	})

	seedDir := t.TempDir()
	seedCfg := transport.Config{
		ListenPort:     reserveTCPPort(t),
		StorePath:      filepath.Join(seedDir, "waku-store.db"),
		PrivateKeyPath: filepath.Join(seedDir, "waku-node-key.json"),
	}

	startSeed := func(t *testing.T) transport.Service {
		t.Helper()
		svc := transport.New(seedCfg)
		require.NoError(t, svc.Start(t.Context()))
		t.Cleanup(func() {
			_ = svc.Stop(t.Context())
		})
		return svc
	}

	var seed transport.Service
	var seedEndpoints []string
	var loopbackSeedEndpoint string
	var rt *testkit.RuntimeHarness

	scenario.Precondition("start seed transport with persistent node key and bootstrapped node", func(t *testing.T) {
		seed = startSeed(t)
		seedEndpoints = append([]string(nil), seed.Endpoints()...)
		loopbackSeedEndpoint = loopbackEndpoint(seedEndpoints)
		require.NotEmpty(t, loopbackSeedEndpoint)

		rt = testkit.NewRuntime(t, runtimeinfra.Config{
			Name: "network-foundation-bootstrap-recovery-e2e",
			Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), seedEndpoints...)},
			Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
		})
		require.NoError(t, rt.Runtime.Start(t.Context()))
		t.Cleanup(func() {
			_ = rt.Runtime.Stop(t.Context())
		})
	})

	scenario.Step("node joins seed bootstrap peer", func(t *testing.T) {
		testkit.WaitForNodeBootstrapReady(t, rt.Runtime)
	})

	scenario.Degraded("seed loss degrades node transport truth", func(t *testing.T) {
		require.NoError(t, seed.Stop(t.Context()))
		testkit.WaitForNodeDegradedAfterPeerLoss(t, rt.Runtime)
	})

	scenario.Step("seed restart preserves endpoints and node recovers", func(t *testing.T) {
		restarted := startSeed(t)
		require.Contains(t, restarted.Endpoints(), loopbackSeedEndpoint)
		seed = restarted

		cooling := testkit.WaitForSnapshot(t, 10*time.Second, rt.Runtime, "boot ready during restricted defense cooldown", func(snap nodeapi.Snapshot) (bool, string) {
			if snap.Boot.State == "ready" && snap.Boot.Joined && snap.Trans.State == "degraded" && snap.Trans.Reason == "restricted defense mode is active" {
				return true, ""
			}
			return false, snap.Boot.State + "/" + snap.Trans.State + "/" + snap.Trans.Reason
		})
		require.Equal(t, "ready", cooling.Boot.State)
		require.Equal(t, "degraded", cooling.Trans.State)

		snap := testkit.WaitForNodeBootstrapRecovery(t, rt.Runtime)
		require.Equal(t, "ready", snap.Boot.State)
		require.Equal(t, "ready", snap.Trans.State)
	})
}

func reserveTCPPort(t *testing.T) int {
	t.Helper()
	return testkit.ReserveLoopbackTCPPort(t)
}

func loopbackEndpoint(endpoints []string) string {
	for _, endpoint := range endpoints {
		if strings.Contains(endpoint, "/ip4/127.0.0.1/") {
			return endpoint
		}
	}
	return ""
}

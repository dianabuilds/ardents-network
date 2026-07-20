//go:build integration

package discovery_test

import (
	"context"
	"testing"
	"time"

	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"
	runtimeinfra "ardents/internal/runtime/process"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestLocalDiscoveryResolveImportedRecord(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	remoteNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "remote",
		Boot: runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	imported := testkit.ImportRecordsFromNode(t, localNode, remoteNode, "bootstrap", nil)
	require.NotEmpty(t, imported)
	rec := imported[0]

	res, err := localNode.ResolveRecord(rec.Subject, rec.Kind)
	require.NoError(t, err)
	require.Equal(t, "found", res.Outcome)
	require.True(t, res.Trust.Valid)
	require.False(t, res.Trust.Trusted)
}

func TestLocalDiscoveryRejectsStaleImport(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})
	rec, private := signedNodeRecord(t, []string{"tcp://fresh"})
	rec.Source = "bootstrap"

	first, err := localNode.ImportRecord(rec)
	require.NoError(t, err)
	require.Equal(t, "completed", first.State)
	require.True(t, first.Accepted)

	stale := rec
	stale.Endpoints = []string{"tcp://older"}
	stale.IssuedAt = rec.IssuedAt.Add(-time.Minute)
	signDiscoveryRecord(t, &stale, private)
	rejected, err := localNode.ImportRecord(stale)
	require.NoError(t, err)
	require.Equal(t, "rejected", rejected.State)
	require.False(t, rejected.Accepted)
}

func TestLocalDiscoveryCandidatesAreNotUsableAfterNodeStop(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	})
	config, endpoint, probe := readyServiceFixture(t)
	remoteNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "remote", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Transport: runtimeinfra.NodeTransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.NodeDataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
		Workload: []runtimeinfra.NodeWorkloadConfig{{
			ID:      "work.remote.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.NodeServiceConfig{{
				ID:             "svc.remote.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, remoteNode, "echo", 1)
	testkit.ImportRecordsFromNode(t, localNode, remoteNode, "bootstrap", nil)
	require.NoError(t, localNode.Stop(context.Background()))

	res, err := localNode.ResolveService("echo")
	require.NoError(t, err)
	require.NotEmpty(t, res.Matches)
	require.Equal(t, "not_found", res.Route.Outcome)
	require.Nil(t, res.Route.Selected)

	items, route, err := localNode.ListRouteCandidates(nodeapi.ListRouteCandidatesQuery{
		Subject: remoteNode.Snapshot().Ident.Principal,
		Kind:    "node",
	})
	require.NoError(t, err)
	require.Len(t, items, 0)
	require.Equal(t, "not_found", route.Outcome)
}

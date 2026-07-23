//go:build integration

package discovery_test

import (
	"context"
	"testing"
	"time"

	nodeapi "ardents/internal/daemon"
	runtimeinfra "ardents/internal/daemon"
	"ardents/internal/discovery"
	transport "ardents/internal/network"
	"ardents/internal/publication"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryDegradesWhenBootstrapPeerIsUnavailable(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	ctx := context.Background()
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))

	remoteTransport := testkit.NewTransport()
	require.NoError(t, remoteTransport.Start(ctx))
	endpoints := remoteTransport.Endpoints()
	require.NoError(t, remoteTransport.Stop(ctx))

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "local",
		Boot:    runtimeinfra.BootConfig{Sources: append([]string(nil), endpoints...)},
		Data:    runtimeinfra.DataConfig{Dir: t.TempDir()},
		Privacy: privacy.Receiver,
	})
	got := n.Snapshot()
	require.Equal(t, "degraded", got.Node.State)
	require.Equal(t, "degraded", got.Trans.State)
	require.NotNil(t, got.Diag.Health.PrimaryReason)
	require.Contains(t, []string{"boot", "transport"}, got.Diag.Health.PrimaryReason.Domain)
	require.False(t, got.Boot.Joined)
	require.Equal(t, "degraded", got.Boot.State)
}

func TestDiscoverySnapshotTracksBootstrapPeerLossAfterStartup(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))

	remoteTransport := testkit.NewTransport()
	require.NoError(t, remoteTransport.Start(ctx))

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "local-loss",
		Boot:    runtimeinfra.BootConfig{Sources: append([]string(nil), remoteTransport.Endpoints()...)},
		Data:    runtimeinfra.DataConfig{Dir: t.TempDir()},
		Privacy: privacy.Receiver,
	})
	require.Equal(t, "ready", n.Snapshot().Boot.State)

	require.NoError(t, remoteTransport.Stop(context.Background()))

	got := testkit.WaitForSnapshot(t, 5*time.Second, n, "degraded boot and transport after peer loss", func(snap nodeapi.SystemSnapshot) (bool, string) {
		if snap.Boot.State == "degraded" && snap.Trans.State == "degraded" {
			return true, ""
		}
		return false, snap.Boot.State + "/" + snap.Trans.State
	})
	require.NotNil(t, got.Diag.Health.PrimaryReason)
	require.Contains(t, []string{"boot", "transport"}, got.Diag.Health.PrimaryReason.Domain)
}

func TestDiscoveryDegradesWhenBootstrapRecordIsInvalid(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Second)
	privacy, remoteTransport := publishInvalidDiscoveryRecord(t, ctx, now)
	defer func() { _ = remoteTransport.Stop(context.Background()) }()

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "local",
		Boot:    runtimeinfra.BootConfig{Sources: append([]string(nil), remoteTransport.Endpoints()...)},
		Data:    runtimeinfra.DataConfig{Dir: t.TempDir()},
		Privacy: privacy.Receiver,
	})
	got := n.Snapshot()
	require.Equal(t, "degraded", got.Node.State)
	require.Equal(t, "degraded", got.Disco.State)
	require.NotNil(t, got.Diag.Health.PrimaryReason)
	require.Equal(t, "discovery", got.Diag.Health.PrimaryReason.Domain)
}

func TestDiscoveryPersistsDegradedStateAcrossRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Second)
	privacy, remoteTransport := publishInvalidDiscoveryRecord(t, ctx, now)

	dir := t.TempDir()
	first := testkit.NewRuntime(t, runtimeinfra.Config{
		Name:    "persist-discovery-degraded",
		Boot:    runtimeinfra.BootConfig{Sources: append([]string(nil), remoteTransport.Endpoints()...)},
		Data:    runtimeinfra.DataConfig{Dir: dir},
		Privacy: privacy.Receiver,
	}).Node
	require.NoError(t, first.Start(context.Background()))
	firstSnap := first.Snapshot()
	require.Equal(t, "degraded", firstSnap.Disco.State)
	require.NotEmpty(t, firstSnap.Disco.Reason)
	require.NoError(t, first.Stop(context.Background()))
	require.NoError(t, remoteTransport.Stop(context.Background()))

	second := testkit.NewRuntime(t, runtimeinfra.Config{
		Name:    "persist-discovery-degraded",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:    runtimeinfra.DataConfig{Dir: dir},
		Privacy: privacy.Receiver,
	}).Node
	require.NoError(t, second.Start(context.Background()))
	defer func() { _ = second.Stop(context.Background()) }()

	got := second.Snapshot()
	require.Equal(t, "degraded", got.Disco.State)
	require.Equal(t, firstSnap.Disco.Reason, got.Disco.Reason)
}

func TestDiscoveryRefreshesLocalPublicationBeforeTTLExpiry(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := readyServiceFixture(t)
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name:                     "refresh-local-discovery",
		NodeProfile:              transport.NodeProfileServiceNode,
		Boot:                     runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport:                runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:                     runtimeinfra.DataConfig{Dir: t.TempDir()},
		DiscoveryRefreshInterval: 50 * time.Millisecond,
		Privacy:                  privacy.Sender,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.work.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, n, "echo", 1)

	records, err := n.ListRecords()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 2)
	initialExpiry := map[string]time.Time{}
	for _, record := range records {
		initialExpiry[record.RecordID()] = record.ExpiresAt
	}

	testkit.WaitForCondition(t, 3*time.Second, "discovery publication refresh before ttl expiry", func() (bool, string) {
		refreshedRecords, err := n.ListRecords()
		if err != nil {
			return false, err.Error()
		}
		for _, record := range refreshedRecords {
			if !record.ExpiresAt.After(initialExpiry[record.RecordID()]) {
				return false, record.RecordID()
			}
		}
		return true, ""
	})
}

func invalidNetworkRecord(t *testing.T, now time.Time) discovery.Record {
	t.Helper()
	record, _ := signedNodeRecord(t, nil)
	record.IssuedAt = now
	record.ExpiresAt = now.Add(time.Hour)
	record.Signature = "aW52YWxpZA=="
	return discovery.RecordFromSnapshot(record)
}

func publishInvalidDiscoveryRecord(t *testing.T, ctx context.Context, now time.Time) (testkit.DiscoveryPrivacyFixture, transport.Service) {
	t.Helper()
	privacy := testkit.NewDiscoveryPrivacyFixture(t, now)
	remoteTransport := testkit.NewTransport()
	require.NoError(t, remoteTransport.Start(ctx))
	require.NoError(t, publication.PublishPrivateDiscoveryEntries(ctx, []discovery.Entry{{
		Record: invalidNetworkRecord(t, now), Source: "local", SeenAt: now,
	}}, privacy.Sender, testkit.PrivateCarrier(remoteTransport)))
	return privacy, remoteTransport
}

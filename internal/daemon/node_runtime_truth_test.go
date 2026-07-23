package daemon

import (
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeStopWithdrawsLocalNodeRecord(t *testing.T) {
	n := NewNode(Config{
		Name: "stop-withdraw",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Start(context.Background()))

	before, err := n.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, before)
	require.NotEmpty(t, before[0].EndpointList())

	require.NoError(t, n.Stop(context.Background()))

	after, err := n.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, after)
	require.Empty(t, after[0].EndpointList())
}

func TestNodeStartSurvivesCallerContextCancellation(t *testing.T) {
	n := NewNode(Config{
		Name: "start-context-independence",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: t.TempDir()},
	})

	ctx, cancel := context.WithCancel(context.Background())
	err := n.Start(ctx)
	require.NoError(t, err)
	cancel()

	snapshot := n.Snapshot()
	require.Equal(t, "degraded", snapshot.Node.State)
	require.NotNil(t, snapshot.Diag.Health.PrimaryReason)
	require.Equal(t, "privacy.capability.missing", snapshot.Diag.Health.PrimaryReason.Code)
	require.True(t, hasSubsystemReason(snapshot.Diag.Health.Subsystems, "data", "privacy.capability.missing"))
	network := n.GetNetworkStatus()
	require.Equal(t, "ardents-private/1", network.PrivacyProfile)
	require.Equal(t, "degraded", network.PrivacyState)
	require.Equal(t, "privacy.capability.missing", network.PrivacySwitchReason)
	require.Equal(t, "blocked", network.PrivacyRecoveryState)
	require.Equal(t, []string{"privacy.capability.missing"}, network.PrivacyErrors)
	require.ElementsMatch(t, []string{"private_publication", "private_discovery", "private_data_exchange"}, network.ReducedCapabilities)

	records, err := n.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.NotEmpty(t, records[0].EndpointList())

	require.NoError(t, n.Stop(context.Background()))
}

func hasSubsystemReason(items []diagapi.SubsystemHealthSnapshot, domain, code string) bool {
	for _, item := range items {
		if item.Domain == domain && item.Reason != nil && item.Reason.Code == code {
			return true
		}
	}
	return false
}

func TestNodeWorkloadReconcilePreservesDiscoveryPrimaryReason(t *testing.T) {
	n := NewNode(Config{
		Name: "workload-primary",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Start(context.Background()))
	defer func() { require.NoError(t, n.Stop(context.Background())) }()

	reason := &diagnostics.Reason{
		Code:                   "discovery.bootstrap.import_degraded",
		Domain:                 "discovery",
		Summary:                "bootstrap discovery import was degraded",
		Detail:                 "broken bootstrap record",
		Impact:                 "remote discovery catalog is incomplete",
		Recovery:               "operator",
		OperatorActionRequired: true,
	}
	n.mu.Lock()
	require.NoError(t, n.disco.Degrade(reason.Detail))
	n.diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	n.diag.SetPrimary(diagnostics.HealthDegraded, reason)
	require.NoError(t, n.life.Move(diagnostics.Degraded))
	err := n.workloadRuntime.Reconcile(context.Background())
	snapshot := n.queryService.DiagnosticsSnapshotLocked()
	n.mu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, snapshot.Health.PrimaryReason)
	require.Equal(t, "discovery", snapshot.Health.PrimaryReason.Domain)
}

func TestNodeRefreshDoesNotClearNonRefreshDiscoveryDegradation(t *testing.T) {
	n := NewNode(Config{
		Name: "refresh-discovery",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Start(context.Background()))
	defer func() { require.NoError(t, n.Stop(context.Background())) }()

	reason := &diagnostics.Reason{
		Code:                   "discovery.bootstrap.import_degraded",
		Domain:                 "discovery",
		Summary:                "bootstrap discovery import was degraded",
		Detail:                 "broken bootstrap record",
		Impact:                 "remote discovery catalog is incomplete",
		Recovery:               "operator",
		OperatorActionRequired: true,
	}
	n.mu.Lock()
	require.NoError(t, n.disco.Degrade(reason.Detail))
	n.diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	n.diag.SetPrimary(diagnostics.HealthDegraded, reason)
	require.NoError(t, n.life.Move(diagnostics.Degraded))
	n.runtimeMgr.RefreshDiscoveryPublicationLocked(context.Background())
	snapshot := n.queryService.SnapshotLocked()
	n.mu.Unlock()

	require.Equal(t, "degraded", snapshot.Disco.State)
	require.NotNil(t, snapshot.Diag.Health.PrimaryReason)
	require.Equal(t, "discovery.bootstrap.import_degraded", snapshot.Diag.Health.PrimaryReason.Code)
}

func TestNodeStopClearsActiveRuntimeDegradation(t *testing.T) {
	n := NewNode(Config{
		Name: "stop-clears-degraded",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: t.TempDir()},
	})
	require.NoError(t, n.Start(context.Background()))

	reason := &diagnostics.Reason{
		Code:                   "boot.join.degraded",
		Domain:                 "boot",
		Summary:                "bootstrap did not complete cleanly",
		Detail:                 "peer unavailable",
		Impact:                 "node remains controllable with limited network confidence",
		Recovery:               "operator",
		OperatorActionRequired: true,
	}
	n.mu.Lock()
	n.boot.SetResult(BootResult{State: "degraded", Reason: reason.Detail})
	require.NoError(t, n.disco.Degrade(reason.Detail))
	n.diag.SetSubsystem("boot", diagnostics.HealthDegraded, reason)
	n.diag.SetPrimary(diagnostics.HealthDegraded, reason)
	require.NoError(t, n.life.Move(diagnostics.Degraded))
	n.mu.Unlock()

	require.NoError(t, n.Stop(context.Background()))

	snapshot := ownersFor(n).Diagnostics.DiagnosticsSnapshot()
	require.Nil(t, snapshot.Health.PrimaryReason)
	for _, item := range snapshot.Health.Subsystems {
		require.False(t, item.Domain == "boot" || item.Domain == "transport" || item.Domain == "discovery")
	}
}

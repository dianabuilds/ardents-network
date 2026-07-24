package daemon

import (
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics"
	networkapi "ardents/internal/network"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNodeStopWaitsForDiscoveryRefreshWriter(t *testing.T) {
	dir := t.TempDir()
	n := NewNode(Config{
		Name: "stop-waits-for-discovery-refresh",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: dir},
	})
	require.NoError(t, n.Start(context.Background()))

	writerStarted := make(chan struct{})
	stopObserved := make(chan struct{})
	releaseWriter := make(chan struct{})
	writerResult := make(chan error, 1)

	n.mu.Lock()
	n.cfg.DiscoveryRefreshInterval = time.Nanosecond
	n.replaceDiscoveryRefreshLocked(func(ctx context.Context) {
		close(writerStarted)
		<-ctx.Done()
		close(stopObserved)
		<-releaseWriter
		writerResult <- os.WriteFile(filepath.Join(dir, "refresh-writer.evidence"), []byte("complete"), 0o600)
	})
	n.mu.Unlock()

	select {
	case <-writerStarted:
	case <-time.After(time.Second):
		t.Fatal("discovery refresh writer did not start")
	}
	stopResult := make(chan error, 1)
	go func() { stopResult <- n.Stop(context.Background()) }()
	select {
	case <-stopObserved:
	case <-time.After(time.Second):
		t.Fatal("Node.Stop did not cancel the discovery refresh writer")
	}
	require.NoError(t, n.Start(context.Background()), "concurrent Start must not reopen lifecycle while Stop is draining")
	select {
	case err := <-stopResult:
		t.Fatalf("Node.Stop returned before the discovery refresh writer stopped: %v", err)
	default:
	}

	close(releaseWriter)
	select {
	case err := <-writerResult:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("discovery refresh writer did not persist its final write")
	}
	select {
	case err := <-stopResult:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Node.Stop did not return after the discovery refresh writer stopped")
	}
	evidence, err := os.ReadFile(filepath.Join(dir, "refresh-writer.evidence"))
	require.NoError(t, err)
	require.Equal(t, "complete", string(evidence))
	n.mu.Lock()
	require.Empty(t, n.refreshLoops)
	n.mu.Unlock()
}

func TestNodeStopDrainsBootstrapDiagnosticsWriter(t *testing.T) {
	dir := t.TempDir()
	n := NewNode(Config{
		Name: "stop-drains-bootstrap-writer",
		Boot: BootConfig{Sources: []string{"local://bootstrap"}},
		Data: DataConfig{Dir: dir},
	})
	require.NoError(t, n.Start(context.Background()))

	n.backgroundMu.Lock()
	stopObserved := n.backgroundStop
	n.backgroundMu.Unlock()
	writerStarted := make(chan struct{})
	releaseWriter := make(chan struct{})
	writerFinished := make(chan struct{})
	go n.runOwnedBackgroundWrite(func() {
		close(writerStarted)
		<-releaseWriter
		RecordBootstrapDial(n.diag, n.cfg.Name, networkapi.BootstrapDialReport{
			Peer:   "/ip4/127.0.0.1/tcp/1",
			Detail: "connection refused",
		})
		close(writerFinished)
	})
	select {
	case <-writerStarted:
	case <-time.After(time.Second):
		t.Fatal("bootstrap diagnostics writer did not start")
	}

	stopResult := make(chan error, 1)
	go func() { stopResult <- n.Stop(context.Background()) }()
	select {
	case <-stopObserved:
	case <-time.After(time.Second):
		t.Fatal("Node.Stop did not close background writer admission")
	}
	select {
	case err := <-stopResult:
		t.Fatalf("Node.Stop returned before the bootstrap diagnostics writer stopped: %v", err)
	default:
	}

	close(releaseWriter)
	select {
	case <-writerFinished:
	case <-time.After(time.Second):
		t.Fatal("bootstrap diagnostics writer did not finish")
	}
	select {
	case err := <-stopResult:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Node.Stop did not return after the bootstrap diagnostics writer finished")
	}
	snapshot := n.diag.Snapshot()
	foundBootstrapFailure := false
	for _, item := range snapshot.RecentEvents {
		foundBootstrapFailure = foundBootstrapFailure || item.Type == "bootstrap_dial_failed"
	}
	require.True(t, foundBootstrapFailure)

	databasePath := filepath.Join(dir, "ardents.db")
	beforeLateCallback, err := os.ReadFile(databasePath)
	require.NoError(t, err)
	n.handleBootstrapDialLocked(networkapi.BootstrapDialReport{
		Peer:   "/ip4/127.0.0.1/tcp/2",
		Detail: "late connection refused",
	})
	afterLateCallback, err := os.ReadFile(databasePath)
	require.NoError(t, err)
	require.Equal(t, beforeLateCallback, afterLateCallback, "late bootstrap observer wrote after Node.Stop")
}

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
	require.NoError(t, n.Stop(context.Background()))
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
	require.Equal(t, "privacy.channel_grant.missing", snapshot.Diag.Health.PrimaryReason.Code)
	require.True(t, hasSubsystemReason(snapshot.Diag.Health.Subsystems, "data", "privacy.channel_grant.missing"))
	network := n.GetNetworkStatus()
	require.Equal(t, "ardents-private/1", network.PrivacyProfile)
	require.Equal(t, "degraded", network.PrivacyState)
	require.Equal(t, "privacy.channel_grant.missing", network.PrivacySwitchReason)
	require.Equal(t, "blocked", network.PrivacyRecoveryState)
	require.Equal(t, []string{"privacy.channel_grant.missing"}, network.PrivacyErrors)
	require.ElementsMatch(t, []networkapi.TransportFeature{
		networkapi.TransportFeaturePrivatePublication,
		networkapi.TransportFeaturePrivateDiscovery,
		networkapi.TransportFeaturePrivateDataExchange,
	}, network.ReducedFeatures)

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

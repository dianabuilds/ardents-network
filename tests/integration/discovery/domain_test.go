//go:build integration

package discovery_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"

	discovery "ardents/internal/discovery"
	transport "ardents/internal/network/api"
	db "ardents/internal/persistence"
	runtimeinfra "ardents/internal/runtime/process"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryResolveQueriesDoNotMutateRouteTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "query-route",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Node
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	defer func() { _ = n.Stop(context.Background()) }()

	before := n.RoutingDetails()
	require.Falsef(t, before.Outcome != "", "initial route outcome = %q, want empty", before.Outcome)

	resolved, err := n.ResolveService("missing")
	require.NoErrorf(t, err, "resolve service: %v", err)
	require.Falsef(t, resolved.Outcome != "not_found", "outcome = %q, want not_found", resolved.Outcome)

	after := n.RoutingDetails()
	require.Falsef(t, after != before, "routing details changed after query: before=%#v after=%#v", before, after)
	{

		snap := n.Snapshot()
		require.Falsef(t, snap.Route.State != "new", "route state = %q, want new", snap.Route.State)
	}
	{

		snap := n.Snapshot()
		require.Falsef(t, snap.Trust.State != "ready", "trust state = %q, want ready local trust baseline", snap.Trust.State)
	}

}

func TestDiscoveryResolveQueriesDoNotMutateTrustTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	localNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "local-trust-query",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Node
	{
		err := localNode.Start(context.Background())
		require.NoErrorf(t, err, "start local node: %v", err)
	}

	defer func() { _ = localNode.Stop(context.Background()) }()

	remoteNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "remote-trust-query",
		Boot: runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Node
	{
		err := remoteNode.Start(context.Background())
		require.NoErrorf(t, err, "start remote node: %v", err)
	}
	defer func() { _ = remoteNode.Stop(context.Background()) }()

	records, err := remoteNode.ListRecords()
	require.NoErrorf(t, err, "list remote records: %v", err)

	record := records[0]
	record.Source = "bootstrap"
	{
		_, err := localNode.ImportRecord(record)
		require.NoErrorf(t, err, "import record: %v", err)
	}

	before := localNode.Snapshot().Trust
	{
		_, err := localNode.ResolveRecord(record.Subject, record.Kind)
		require.NoErrorf(t, err, "resolve record: %v", err)
	}

	after := localNode.Snapshot().Trust
	require.Falsef(t, after != before, "trust snapshot changed after resolve query: before=%#v after=%#v", before, after)

}

func TestDiscoveryResolveImportedRecord(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	localNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Node
	{
		err := localNode.Start(context.Background())
		require.NoErrorf(t, err, "start local node: %v", err)
	}

	defer func() { _ = localNode.Stop(context.Background()) }()

	remoteNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "remote",
		Boot: runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Node
	{
		err := remoteNode.Start(context.Background())
		require.NoErrorf(t, err, "start remote node: %v", err)
	}
	defer func() { _ = remoteNode.Stop(context.Background()) }()

	records, err := remoteNode.ListRecords()
	require.NoErrorf(t, err, "list remote records: %v", err)

	record := records[0]
	record.Source = "bootstrap"
	{

		_, err := localNode.ImportRecord(record)
		require.NoErrorf(t, err, "import record: %v", err)
	}

	result, err := localNode.ResolveRecord(record.Subject, record.Kind)
	require.NoErrorf(t, err, "resolve record: %v", err)
	require.Falsef(t, result.Outcome != "found", "outcome = %q, want found", result.Outcome)
	require.True(t, result.Trust.Valid, "expected valid record")
	require.False(t, result.Trust.Trusted, "did not expect imported record to be trusted")

}

func TestDiscoveryResolveRecordRejectsExpiredPersistedRecord(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	now := time.Now().UTC()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	record := discovery.Record{
		ID:        "expired-remote:node",
		Kind:      "node",
		Subject:   "expired-remote",
		Node:      "expired-remote",
		Device:    "expired-device",
		PublicKey: base64.StdEncoding.EncodeToString(public),
		Endpoints: []string{"tcp://expired-remote:9000"},
		IssuedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	}
	payload, err := discovery.Canonical(record)
	require.NoErrorf(t, err, "canonical expired record: %v", err)

	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	entry := discovery.Entry{
		Record: record,
		Source: "cache",
		SeenAt: now,
	}
	{
		err := db.SaveJSON(filepath.Join(dir, "ardents.db"), "discovery", "records", map[string]any{
			"records": []discovery.Entry{entry},
			"state":   "ready",
		})
		require.NoErrorf(t, err, "save expired records: %v", err)
	}

	localNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "local-expired",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: dir},
	}).Node
	{
		err := localNode.Start(context.Background())
		require.NoErrorf(t, err, "start local node: %v", err)
	}

	defer func() { _ = localNode.Stop(context.Background()) }()
	beforeSnapshot := localNode.Snapshot()

	result, err := localNode.ResolveRecord("expired-remote", "node")
	require.NoErrorf(t, err, "resolve expired record: %v", err)
	require.Falsef(t, result.Outcome != "expired", "outcome = %q, want expired", result.Outcome)
	require.Falsef(t, len(result.Candidates) !=
		0, "candidates = %d, want 0 for expired record", len(result.Candidates))
	require.Nilf(t, result.Route.Selected, "selected route = %#v, want nil", result.Route.Selected)
	require.Falsef(t, result.Route.Candidates !=
		0, "route candidates = %d, want 0", result.Route.Candidates)
	require.Falsef(t, result.Trust.Outcome !=
		"expired", "trust outcome = %q, want expired", result.Trust.Outcome)

	snapshot := localNode.Snapshot()
	require.Falsef(t, snapshot.Disco.Records !=
		beforeSnapshot.Disco.
			Records, "snapshot records = %d, want unchanged visible count %d", snapshot.Disco.Records, beforeSnapshot.Disco.Records)
	require.Falsef(t, snapshot.Disco.Services !=
		0, "snapshot services = %d, want 0 visible discovery services", snapshot.Disco.Services)

}

func TestDiscoveryStatusCountsExpiredRecordAsStaleAndRejected(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	now := time.Now().UTC()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	record := discovery.Record{
		ID:        "expired-summary:node",
		Kind:      "node",
		Subject:   "expired-summary",
		Node:      "expired-summary",
		Device:    "expired-device",
		PublicKey: base64.StdEncoding.EncodeToString(public),
		Endpoints: []string{"tcp://expired-summary:9000"},
		IssuedAt:  now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Minute),
	}
	payload, err := discovery.Canonical(record)
	require.NoErrorf(t, err, "canonical expired record: %v", err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))

	entry := discovery.Entry{
		Record: record,
		Source: "cache",
		SeenAt: now,
	}
	{
		err := db.SaveJSON(filepath.Join(dir, "ardents.db"), "discovery", "records", map[string]any{
			"records": []discovery.Entry{entry},
			"state":   "ready",
		})
		require.NoErrorf(t, err, "save expired records: %v", err)
	}

	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name: "local-expired-summary",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: dir},
	})

	status := localNode.GetDiscoveryStatus()
	require.Equal(t, 1, status.RemoteRecords)
	require.Equal(t, 1, status.StaleRecords)
	require.Equal(t, 1, status.RejectedRecords)
}

func TestDiscoveryDoesNotPublishStaticServiceRecordWithoutRuntimeBacking(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "svc",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
		Service: []runtimeinfra.NodeServiceConfig{{
			ID:        "svc.local.echo",
			Type:      "echo",
			Owner:     "node",
			Mode:      "NetworkPublished",
			Endpoints: []string{"tcp://127.0.0.1:9000"},
		}},
	}).Node
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}

	defer func() { _ = n.Stop(context.Background()) }()

	result, err := n.ResolveRecord("svc.local.echo", "service")
	require.NoErrorf(t, err, "resolve service record: %v", err)
	require.Falsef(t, result.Outcome != "not_found", "outcome = %q, want not_found for static service without runtime backing", result.Outcome)

}

func TestDiscoveryResolveServiceType(t *testing.T) {
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
	localNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	}).Node
	{
		err := localNode.Start(context.Background())
		require.NoErrorf(t, err, "start local node: %v", err)
	}

	defer func() { _ = localNode.Stop(context.Background()) }()

	config, endpoint, probe := readyServiceFixture(t)
	remoteNode := testkit.NewRuntime(t, runtimeinfra.Config{
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
	}).Node
	{
		err := remoteNode.Start(context.Background())
		require.NoErrorf(t, err, "start remote node: %v", err)
	}

	defer func() { _ = remoteNode.Stop(context.Background()) }()

	testkit.WaitForServiceMatchCount(t, 10*time.Second, remoteNode, "echo", 1)
	records, err := remoteNode.ListRecords()
	require.NoErrorf(t, err, "list remote records: %v", err)

	for _, record := range records {
		if record.Kind != "service" {
			continue
		}
		record.Source = "bootstrap"
		{
			_, err := localNode.ImportRecord(record)
			require.NoErrorf(t, err, "import service record: %v", err)
		}

	}

	result, err := localNode.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service type: %v", err)
	require.False(t, len(result.Matches) ==
		0, "expected service matches")
	require.NotNil(t, result.Route.Selected, "expected selected route")
	require.Falsef(t, result.Outcome != "not_usable", "outcome = %q, want not_usable for untrusted service", result.Outcome)
	require.Equal(t, "http", result.Route.Selected.Scheme)

}

func TestDiscoveryImportRecordRejectsStaleRecordWithoutPublishingSuccess(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "discovery",
		ScenarioID:  "DKI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "discovery"},
		Speed:       "default",
		Environment: "local",
	})
	localNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	}).Node
	{
		err := localNode.Start(context.Background())
		require.NoErrorf(t, err, "start local node: %v", err)
	}

	defer func() { _ = localNode.Stop(context.Background()) }()

	record, private := signedNodeRecord(t, []string{"tcp://fresh"})
	record.Source = "bootstrap"
	first, err := localNode.ImportRecord(record)
	require.NoErrorf(t, err, "import fresh record: %v", err)
	require.Falsef(t, first.State != "completed" ||
		!first.Accepted, "status = %#v, want completed accepted", first)

	stale := record
	stale.Endpoints = []string{"tcp://older"}
	stale.IssuedAt = record.IssuedAt.Add(-time.Minute)
	signDiscoveryRecord(t, &stale, private)
	result, err := localNode.ImportRecord(stale)
	require.NoErrorf(t, err, "import stale record: %v", err)
	require.Falsef(t, result.State !=
		"rejected" ||
		result.Accepted, "status = %#v, want rejected not accepted", result)

	resolved, err := localNode.ResolveRecord(record.Subject, record.Kind)
	require.NoErrorf(t, err, "resolve record: %v", err)
	require.Falsef(t, len(resolved.Record.Endpoints) != len(record.Endpoints) ||
		strings.Join(resolved.Record.
			Endpoints, ",",
		) != strings.Join(
			record.Endpoints,
			","), "endpoints = %v, want original %v", resolved.Record.Endpoints, record.Endpoints)

}
func TestDiscoveryResolveRecordAndServiceDoNotReturnUsableRoutesAfterStop(t *testing.T) {
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
	localNode := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "local",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	}).Node
	{
		err := localNode.Start(context.Background())
		require.NoErrorf(t, err, "start local node: %v", err)
	}

	config, endpoint, probe := readyServiceFixture(t)
	remoteNode := testkit.NewRuntime(t, runtimeinfra.Config{
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
	}).Node
	{
		err := remoteNode.Start(context.Background())
		require.NoErrorf(t, err, "start remote node: %v", err)
	}

	defer func() { _ = remoteNode.Stop(context.Background()) }()

	testkit.WaitForServiceMatchCount(t, 10*time.Second, remoteNode, "echo", 1)
	records, err := remoteNode.ListRecords()
	require.NoErrorf(t, err, "list remote records: %v", err)

	for _, record := range records {
		record.Source = "bootstrap"
		{
			_, err := localNode.ImportRecord(record)
			require.NoErrorf(t, err, "import record: %v", err)
		}

	}
	{

		err := localNode.Stop(context.Background())
		require.NoErrorf(t, err, "stop local node: %v", err)
	}

	recordRes, err := localNode.ResolveRecord(remoteNode.Snapshot().Ident.Principal, "node")
	require.NoErrorf(t, err, "resolve record after stop: %v", err)
	require.Falsef(t, recordRes.Outcome != "found", "outcome = %q, want found", recordRes.Outcome)
	require.Falsef(t, len(recordRes.Candidates) != 0, "candidates = %d, want 0 after stop", len(recordRes.Candidates))
	require.Falsef(t, recordRes.Route.Outcome !=
		"not_found", "route outcome = %q, want not_found", recordRes.Route.Outcome)
	require.Nil(t, recordRes.Route.Selected, "expected no selected route after stop")
	{

		snapshot := localNode.Snapshot()
		require.Falsef(t, snapshot.Route.State !=
			"new", "route state after stop query = %q, want new", snapshot.Route.State)
	}

	serviceRes, err := localNode.ResolveService("echo")
	require.NoErrorf(t, err, "resolve service after stop: %v", err)
	require.False(t, len(serviceRes.Matches) ==
		0, "expected discovery match after stop")
	require.Falsef(t, serviceRes.Route.Outcome !=
		"not_found", "service route outcome = %q, want not_found", serviceRes.Route.Outcome)
	require.Nil(t, serviceRes.Route.Selected, "expected no selected service route after stop")

	for _, match := range serviceRes.Matches {
		require.Falsef(t, len(match.Candidates) !=
			0, "match candidates = %d, want 0 after stop", len(match.Candidates))

	}
	{
		snapshot := localNode.Snapshot()
		require.Falsef(t, snapshot.Route.State !=
			"new", "route state after service query = %q, want new", snapshot.Route.State)
	}

}

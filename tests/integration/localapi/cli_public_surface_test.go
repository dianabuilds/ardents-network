//go:build integration

package localapi_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"ardents/internal/cli"
	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	rpcadapter "ardents/internal/localapi"
	transport "ardents/internal/network"
	"ardents/internal/transfer"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestCLINetworkPublicSurfaceReflectsLocalTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-NET-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "cli", "network", "discovery"},
		Speed:       "default",
		Environment: "local",
	})

	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	config, endpoint, probe := localControlReadyFixture(t)
	remote := testkit.StartNode(t, runtimeinfra.Config{
		Name: "cli-network-surface-remote", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.remote.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  config,
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.remote.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{endpoint},
				ProbeEndpoints: []string{probe},
			}},
		}},
	})
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name:        "cli-network-surface-local",
		NodeProfile: transport.NodeProfileServiceNode,
		Boot:        runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Trust:       runtimeinfra.TrustConfig{Anchors: []string{remote.Snapshot().Ident.PublicKey}},
		Transport:   runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:        runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	})
	testkit.WaitForServiceMatchCount(t, 10*time.Second, remote, "echo", 1)
	imported := testkit.ImportRecordsFromNode(t, localNode, remote, "bootstrap", nil)
	require.NotEmpty(t, imported)

	network := localNode.GetNetworkStatus()
	discovery := localNode.GetDiscoveryStatus()
	peers := localNode.ListPeers()
	routes, route, err := localNode.ListRouteCandidates(discoveryapi.ListRouteCandidatesQuery{Service: "echo"})
	require.NoError(t, err)

	cliHarness := newCLIHarness(t, localNode)

	networkOut := cliHarness.run(t, "network", "status")
	require.Contains(t, networkOut.stdout, "network status")
	require.Contains(t, networkOut.stdout, "state: "+network.State)
	require.Contains(t, networkOut.stdout, "joined: "+boolString(network.Joined))
	require.Contains(t, networkOut.stdout, "reachable: "+boolString(network.Reachable))
	require.Contains(t, networkOut.stdout, "profile: "+network.ActiveProfile)
	require.Contains(t, networkOut.stdout, "mode: "+network.ActiveMode)
	require.Contains(t, networkOut.stdout, "node_profile: "+network.NodeProfile)
	require.Contains(t, networkOut.stdout, "privacy_profile: "+network.PrivacyProfile)
	require.Contains(t, networkOut.stdout, "privacy_state: "+network.PrivacyState)
	require.Contains(t, networkOut.stdout, "privacy_switch_reason: "+network.PrivacySwitchReason)
	require.Contains(t, networkOut.stdout, "privacy_recovery_state: "+network.PrivacyRecoveryState)

	discoveryOut := cliHarness.run(t, "network", "discovery")
	require.Contains(t, discoveryOut.stdout, "network discovery")
	require.Contains(t, discoveryOut.stdout, "state: "+discovery.State)
	require.Contains(t, discoveryOut.stdout, "local_records: "+itoa(discovery.LocalRecords))
	require.Contains(t, discoveryOut.stdout, "remote_records: "+itoa(discovery.RemoteRecords))
	require.Contains(t, discoveryOut.stdout, "trusted_records: "+itoa(discovery.TrustedRecords))

	peersOut := cliHarness.run(t, "network", "peers")
	require.NotEmpty(t, peers)
	require.Contains(t, peersOut.stdout, "network peers")
	require.Contains(t, peersOut.stdout, "peer: "+peers[0].NodeID)
	require.Contains(t, peersOut.stdout, "  state: "+peers[0].State)
	require.Contains(t, peersOut.stdout, "  trust: "+peers[0].Trust.State)

	routesOut := cliHarness.run(t, "network", "routes", "--service", "echo")
	require.NotEmpty(t, routes)
	require.Contains(t, routesOut.stdout, "network routes")
	require.Contains(t, routesOut.stdout, "outcome: "+route.Outcome)
	require.Contains(t, routesOut.stdout, "candidate: "+routes[0].Endpoint)
	require.Contains(t, routesOut.stdout, "  usable: "+boolString(routes[0].Usable))
}

func TestCLIDiagnosticsSurfaceExplainsDegradedTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-DIAG-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "cli", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "cli-diagnostics-surface",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.invalid",
			Kind:    "unsupported",
			Owner:   "node",
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.invalid",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://127.0.0.1:9000"},
			}},
		}},
	})

	health := rt.Diagnostics.GetHealthSummary()
	explanation := rt.Diagnostics.ExplainFailure("service", "svc.work.invalid")
	cliHarness := newCLIHarness(t, rt.Runtime)

	healthOut := cliHarness.run(t, "diagnostics", "health")
	require.Contains(t, healthOut.stdout, "diagnostics health")
	require.Contains(t, healthOut.stdout, "state: "+health.State)
	require.NotNil(t, health.PrimaryReason)
	require.Contains(t, healthOut.stdout, "reason: "+health.PrimaryReason.Summary)
	require.Contains(t, healthOut.stdout, "operator_action_required: "+boolString(health.OperatorActionRequired))

	explainOut := cliHarness.run(t, "diagnostics", "explain", "--scope", "service", "--resource-id", "svc.work.invalid")
	require.Contains(t, explainOut.stdout, "diagnostics explain")
	require.Contains(t, explainOut.stdout, "scope: "+explanation.Scope)
	require.Contains(t, explainOut.stdout, "resource_id: "+explanation.ResourceID)
	require.Contains(t, explainOut.stdout, "state: "+explanation.State)
	require.NotNil(t, explanation.Reason)
	require.Contains(t, explainOut.stdout, "reason: "+explanation.Reason.Summary)
	require.Contains(t, explainOut.stdout, "recovery: "+explanation.Reason.Recovery)

	eventsOut := cliHarness.run(t, "diagnostics", "events", "--limit", "2")
	events, _ := rt.Diagnostics.ListRecentEvents(2, "")
	require.Contains(t, eventsOut.stdout, "diagnostics events")
	require.NotEmpty(t, events)
	require.Contains(t, eventsOut.stdout, events[len(events)-1].Type)
}

func TestCLIHostedServiceSurfaceTracksWorkloadPublicationTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-SVC-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "cli", "workload", "hosted-service"},
		Speed:       "default",
		Environment: "local",
	})

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "cli-service-surface",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  helperProcessConfig(t, "sleep"),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:        "svc.work.echo",
				Type:      "echo",
				Mode:      "NetworkPublished",
				Endpoints: []string{"tcp://echo:9000"},
			}},
		}},
	})

	services, err := rt.Hosting.ListHostedServices()
	require.NoError(t, err)
	service, err := rt.Hosting.GetHostedService("svc.work.echo")
	require.NoError(t, err)
	publication, err := rt.Hosting.GetServicePublicationStatus("svc.work.echo")
	require.NoError(t, err)

	cliHarness := newCLIHarness(t, rt.Runtime)

	servicesOut := cliHarness.run(t, "workload", "services")
	require.Contains(t, servicesOut.stdout, "workload services")
	require.NotEmpty(t, services)
	require.Contains(t, servicesOut.stdout, "service: "+services[0].ID)
	require.Contains(t, servicesOut.stdout, "  runtime_backing: "+services[0].RuntimeBacking)
	require.Contains(t, servicesOut.stdout, "  readiness: "+services[0].Readiness)
	require.Contains(t, servicesOut.stdout, "  ready: "+boolString(services[0].Ready))
	require.Contains(t, servicesOut.stdout, "  exposure_eligible: "+boolString(services[0].ExposureEligible))

	serviceOut := cliHarness.run(t, "workload", "service", "svc.work.echo")
	require.Contains(t, serviceOut.stdout, "workload service")
	require.Contains(t, serviceOut.stdout, "state: "+service.State)
	require.Contains(t, serviceOut.stdout, "published: "+boolString(service.Published))
	require.Contains(t, serviceOut.stdout, "runtime_backing: "+service.RuntimeBacking)
	require.Contains(t, serviceOut.stdout, "ready: "+boolString(service.Ready))
	require.Contains(t, serviceOut.stdout, "exposure_eligible: "+boolString(service.ExposureEligible))

	publicationOut := cliHarness.run(t, "workload", "publication", "svc.work.echo")
	require.Contains(t, publicationOut.stdout, "workload publication")
	require.Contains(t, publicationOut.stdout, "state: "+publication.State)
	require.Contains(t, publicationOut.stdout, "published: "+boolString(publication.Published))

	stopOut := cliHarness.run(t, "workload", "stop", "work.echo")
	require.Contains(t, stopOut.stdout, "workload stop")
	require.Contains(t, stopOut.stdout, "workload: work.echo")

	serviceAfterStop, err := rt.Hosting.GetHostedService("svc.work.echo")
	require.NoError(t, err)
	publicationAfterStop, err := rt.Hosting.GetServicePublicationStatus("svc.work.echo")
	require.NoError(t, err)

	serviceAfterStopOut := cliHarness.run(t, "workload", "service", "svc.work.echo")
	require.Contains(t, serviceAfterStopOut.stdout, "published: "+boolString(serviceAfterStop.Published))
	require.Contains(t, serviceAfterStopOut.stdout, "reason: "+serviceAfterStop.Reason)

	publicationAfterStopOut := cliHarness.run(t, "workload", "publication", "svc.work.echo")
	require.Contains(t, publicationAfterStopOut.stdout, "state: "+publicationAfterStop.State)
	require.Contains(t, publicationAfterStopOut.stdout, "reason: "+publicationAfterStop.Reason)
	require.Contains(t, publicationAfterStopOut.stdout, "published: "+boolString(publicationAfterStop.Published))
}

func TestCLIDataTransferSurfaceReflectsFetchRuntimeTruth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-DATA-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "cli", "data", "transfer"},
		Speed:       "default",
		Environment: "local",
	})

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := []byte("0123456789abcdef0123456789abcdef")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoError(t, err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "cli-data-surface-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
	})
	records, err := source.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name:  "cli-data-surface-requester",
		Boot:  runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].EndpointList()...)},
		Trust: runtimeinfra.TrustConfig{Anchors: []string{source.Snapshot().Ident.PublicKey}},
		Data:  runtimeinfra.DataConfig{Dir: t.TempDir()},
	})

	cliHarness := newCLIHarness(t, rt.Runtime)

	fetchOut := cliHarness.run(t, "data", "blobs", "fetch", stored.ID)
	require.Contains(t, fetchOut.stdout, "data blob fetch")
	require.Contains(t, fetchOut.stdout, "blob: "+stored.ID)

	var transfers []transfer.Record
	testkit.WaitForCondition(t, 5*time.Second, "cli data transfer becomes visible through local truth", func() (bool, string) {
		transfers = rt.Transfers.List()
		transferRecord, ok := findLocalTransferByResource(transfers, stored.ID)
		if !ok {
			return false, "local transfer not visible yet"
		}
		if transferRecord.State != "completed" {
			return false, transferRecord.State
		}
		return true, ""
	})

	sources := rt.Data.ListBlobSources(stored.ID)
	transferRecord, ok := findLocalTransferByResource(transfers, stored.ID)
	require.True(t, ok)
	detail, ok := rt.Transfers.Get(transferRecord.ID)
	require.True(t, ok)
	inventory := rt.Data.InventorySnapshot()

	sourcesOut := cliHarness.run(t, "data", "blobs", "sources", stored.ID)
	require.Contains(t, sourcesOut.stdout, "data blob sources")
	require.GreaterOrEqual(t, len(sources), 2)
	localSource, ok := findLocalBlobSourceByTransport(sources, "local")
	require.True(t, ok)
	remoteSource, ok := findLocalBlobSourceByTransport(sources, "remote")
	require.True(t, ok)
	require.Contains(t, sourcesOut.stdout, "source: "+localSource.NodeID)
	require.Contains(t, sourcesOut.stdout, "source: "+remoteSource.NodeID)
	require.Contains(t, sourcesOut.stdout, "  usable: "+boolString(localSource.Usable))

	transfersOut := cliHarness.run(t, "data", "transfers", "list")
	require.Contains(t, transfersOut.stdout, "data transfers")
	require.Contains(t, transfersOut.stdout, "transfer: "+transferRecord.ID)
	require.Contains(t, transfersOut.stdout, "  state: "+transferRecord.State)
	require.Contains(t, transfersOut.stdout, "  resource: "+transferRecord.ResourceID)

	transferOut := cliHarness.run(t, "data", "transfers", "get", transferRecord.ID)
	require.Contains(t, transferOut.stdout, "data transfer")
	require.Contains(t, transferOut.stdout, "transfer: "+detail.ID)
	require.Contains(t, transferOut.stdout, "  state: "+detail.State)
	require.Contains(t, transferOut.stdout, "  resource: "+detail.ResourceID)

	inventoryOut := cliHarness.run(t, "data", "inventory")
	require.Contains(t, inventoryOut.stdout, "data inventory")
	require.Contains(t, inventoryOut.stdout, "local_blobs: "+itoa(inventory.LocalBlobs))
}

func TestCLIDataSurfaceShowsStaleRemoteSourceAsUnusable(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "local-control-surface",
		ScenarioID:  "INT-DATA-SURFACE-001",
		Suite:       "integration",
		Tags:        []string{"integration", "local-control-surface", "cli", "data", "transfer"},
		Speed:       "default",
		Environment: "local",
	})

	dataDir := t.TempDir()
	store := appdata.NewInDir(dataDir)
	require.NoError(t, store.Load())
	blob, err := store.AnnounceRemoteBlob(appdata.Blob{
		ID:        "blob-cli-stale-remote",
		CID:       "blob-cli-stale-remote",
		MediaType: "application/octet-stream",
		State:     "available-remote",
	})
	require.NoError(t, err)
	_, err = store.ObserveBlobSource(blob.ID, appdata.BlobSourceRecord{
		NodeID:     "p_remote_stale_cli",
		Trust:      appdata.SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:     true,
		Transport:  "remote",
		LastSeenAt: time.Now().UTC().Add(-24 * time.Hour),
		Reason:     "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)

	rt := testkit.StartRuntime(t, runtimeinfra.Config{
		Name: "cli-data-surface-stale-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dataDir},
	})

	sources := rt.Data.ListBlobSources(blob.ID)
	require.Len(t, sources, 1)
	require.False(t, sources[0].Usable)
	require.Contains(t, sources[0].Reason, "stale")

	cliHarness := newCLIHarness(t, rt.Runtime)
	sourcesOut := cliHarness.run(t, "data", "blobs", "sources", blob.ID)
	require.Contains(t, sourcesOut.stdout, "data blob sources")
	require.Contains(t, sourcesOut.stdout, "source: "+sources[0].NodeID)
	require.Contains(t, sourcesOut.stdout, "  usable: false")
	require.Contains(t, sourcesOut.stdout, "stale")
}

type cliHarness struct {
	addr  string
	token string
}

type cliResult struct {
	stdout string
	stderr string
	code   int
}

func newCLIHarness(t *testing.T, runtime *runtimeprocess.Node) cliHarness {
	t.Helper()

	mux := http.NewServeMux()
	auth := testkit.ConnectAuthConfig()
	path, handler, err := rpcadapter.NewHandler(testkit.ConnectDependencies(runtime), auth)
	require.NoError(t, err)
	mux.Handle(path, handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return cliHarness{addr: srv.URL, token: auth.Token}
}

func (h cliHarness) run(t *testing.T, args ...string) cliResult {
	t.Helper()

	t.Setenv("ARDENTS_ADDR", h.addr)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", h.token)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(context.Background(), args, &stdout, &stderr)
	require.Equalf(t, 0, code, "stderr=%s", stderr.String())
	expectedWarning := "ardentsctl: warning: explicit legacy bearer authentication is migration-only\n"
	for index, argument := range args {
		if argument == "--output=json" || (argument == "--output" && index+1 < len(args) && args[index+1] == "json") {
			expectedWarning = "{\"warning\":\"legacy bearer authentication is migration-only\"}\n"
		}
	}
	require.Equal(t, expectedWarning, stderr.String())

	return cliResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

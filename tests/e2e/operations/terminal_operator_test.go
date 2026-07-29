//go:build e2e

package operations_e2e_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/internal/cli"
	"ardents/internal/cli/catalog"
	cliclient "ardents/internal/cli/client"
	appdata "ardents/internal/content"
	contentpayload "ardents/internal/content/payload"
	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	discoveryrecords "ardents/internal/discovery/records"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/localapi/protocol"
	transport "ardents/internal/network"
	"ardents/internal/storage"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTerminalReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_TERMINAL_READY_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_TERMINAL_READY_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

func TestTerminalNodeRuntimeLifecycleAcrossRestartPreservesPendingTruth(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "NRE-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "node", "diagnostics", "ocs-02"},
		Speed:       "default",
		Environment: "local",
	})

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "operations.json"), []byte(seedRecoverableOperation), 0o644))
	cfg := runtimeinfra.Config{
		Name: "terminal-node-runtime-e2e",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}

	first := testkit.NewRuntime(t, cfg).Runtime
	t.Cleanup(func() { _ = first.Stop(context.Background()) })
	firstTerminal := newTerminalHarness(t, first)

	scenario.Precondition("operator starts node through terminal command", func(t *testing.T) {
		start := firstTerminal.run(t, context.Background(), "node", "start")
		require.Contains(t, start.stdout, "node start")
		require.Contains(t, start.stdout, "status: completed")
	})

	scenario.Step("terminal shows ready runtime and recovering pending operation after startup", func(t *testing.T) {
		status := firstTerminal.run(t, context.Background(), "node", "status")
		require.Contains(t, status.stdout, "node status")
		require.Contains(t, status.stdout, "state: ready")
		require.Contains(t, status.stdout, "ready: true")
		requireProtoJSONFields(t, firstTerminal.run(t, context.Background(), "--output", "json", "node", "status"), "status", "snapshot", "features")

		directPending := testkit.Diagnostics(first).PendingOperations()
		require.NotEmpty(t, directPending, "direct diagnostics must retain recovery truth before terminal projection")
		pending := firstTerminal.run(t, context.Background(), "diagnostics", "pending")
		require.Contains(t, pending.stdout, "diagnostics pending")
		require.Contains(t, pending.stdout, "operation: op-1")
		require.Contains(t, pending.stdout, "  state: recovering")
		require.Contains(t, pending.stdout, "  recovery_action: restart node")

		runtime := firstTerminal.run(t, context.Background(), "node", "runtime")
		require.Contains(t, runtime.stdout, "node runtime")
		require.Contains(t, runtime.stdout, "state: ready")
		require.Contains(t, runtime.stdout, "principal: "+firstTerminal.nodePrincipal)
		requireProtoJSONFields(t, firstTerminal.run(t, context.Background(), "--output", "json", "node", "runtime"), "status", "runtime")

		features := firstTerminal.run(t, context.Background(), "--output", "json", "node", "features")
		var featureResponse ardentsv1.NodeFeaturesResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(features.stdout), &featureResponse), "stdout=%s", features.stdout)
		require.NotEmpty(t, featureResponse.GetFeatures().GetVersion())
		require.NotEmpty(t, featureResponse.GetFeatures().GetServices())
		featuresHuman := firstTerminal.run(t, context.Background(), "node", "features")
		require.Contains(t, featuresHuman.stdout, "node features")
		require.Contains(t, featuresHuman.stdout, "version:")

		snapshot := firstTerminal.run(t, context.Background(), "--output", "json", "diagnostics", "snapshot")
		var diagnosticResponse ardentsv1.DiagnosticsSnapshotResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(snapshot.stdout), &diagnosticResponse), "stdout=%s", snapshot.stdout)
		require.True(t, diagnosticResponse.GetStatus().GetAccepted())
		require.NotEmpty(t, diagnosticResponse.GetDiagnostics().GetPendingOperations())
		snapshotHuman := firstTerminal.run(t, context.Background(), "diagnostics", "snapshot")
		require.Contains(t, snapshotHuman.stdout, "diagnostics snapshot")
		require.Contains(t, snapshotHuman.stdout, "pending_operations:")

		health := firstTerminal.run(t, context.Background(), "diagnostics", "health")
		require.Contains(t, health.stdout, "diagnostics health")
		require.Contains(t, health.stdout, "state:")
		requireProtoJSONFields(t, firstTerminal.run(t, context.Background(), "--output", "json", "diagnostics", "health"), "status", "health")

		requireProtoJSONFields(t, firstTerminal.run(t, context.Background(), "--output", "json", "diagnostics", "pending"), "status", "operations")

		explain := firstTerminal.run(t, context.Background(), "diagnostics", "explain", "--scope", "workload", "--resource-id", "workloads")
		require.Contains(t, explain.stdout, "diagnostics explain")
		require.Contains(t, explain.stdout, "resource_id: workloads")
		requireProtoJSONFields(t, firstTerminal.run(t, context.Background(), "--output", "json", "diagnostics", "explain", "--scope", "workload", "--resource-id", "workloads"), "status", "explanation")

		events := firstTerminal.run(t, context.Background(), "--output", "json", "diagnostics", "events", "--limit", "5")
		var eventResponse ardentsv1.ListEventsResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(events.stdout), &eventResponse), "stdout=%s", events.stdout)
		require.True(t, eventResponse.GetStatus().GetAccepted())
		require.NotEmpty(t, eventResponse.GetEvents())
		eventsHuman := firstTerminal.run(t, context.Background(), "diagnostics", "events", "--limit", "5")
		require.Contains(t, eventsHuman.stdout, "diagnostics events")
	})

	scenario.Step("terminal stop persists shutdown terminal fate", func(t *testing.T) {
		stop := firstTerminal.run(t, context.Background(), "node", "stop")
		require.Contains(t, stop.stdout, "node stop")
		require.Contains(t, stop.stdout, "status: completed")

		raw, err := os.ReadFile(filepath.Join(dir, "operations.json"))
		require.NoError(t, err)
		require.Contains(t, string(raw), `"kind": "node.shutdown"`)
		require.Contains(t, string(raw), `"state": "completed"`)
	})

	second := testkit.NewRuntime(t, cfg).Runtime
	t.Cleanup(func() { _ = second.Stop(context.Background()) })
	secondTerminal := newTerminalHarness(t, second)

	scenario.Step("operator restarts node from persisted state through terminal command", func(t *testing.T) {
		start := secondTerminal.run(t, context.Background(), "node", "start")
		require.Contains(t, start.stdout, "node start")
		require.Contains(t, start.stdout, "status: completed")
	})

	scenario.Assert("restart keeps recovery truth visible through terminal diagnostics", func(t *testing.T) {
		status := secondTerminal.run(t, context.Background(), "node", "status")
		require.Contains(t, status.stdout, "state: ready")
		require.Contains(t, status.stdout, "ready: true")

		pending := secondTerminal.run(t, context.Background(), "diagnostics", "pending")
		require.Contains(t, pending.stdout, "operation: op-1")
		require.Contains(t, pending.stdout, "  state: recovering")
		require.Contains(t, pending.stdout, "  recovery_action: restart node")
	})
}

func TestTerminalNetworkSurfaceLifecycle(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "E2E-NET-SURFACE-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "network", "discovery", "ocs-02"},
		Speed:       "default",
		Environment: "local",
	})

	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	remotePort := reserveTerminalPort(t)
	remoteEndpoint := terminalAdvertisedEndpoint(t, remotePort)
	remote := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-network-remote", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender, DiscoveryRefreshInterval: 50 * time.Millisecond,
		Workload: []runtimeinfra.WorkloadConfig{{
			ID:      "work.remote.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  terminalReadyConfig(t, remotePort),
			Desired: "running",
			Services: []runtimeinfra.ServiceConfig{{
				ID:             "svc.remote.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{remoteEndpoint},
				ProbeEndpoints: []string{remoteEndpoint},
			}},
		}},
	}).Node
	require.NoError(t, remote.Start(context.Background()))
	t.Cleanup(func() { _ = remote.Stop(context.Background()) })

	sut := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-network-sut", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Trust:     runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, remote.Snapshot().Ident.PublicKey)},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	}).Runtime
	t.Cleanup(func() { _ = sut.Stop(context.Background()) })
	terminal := newTerminalHarness(t, sut)
	var importedNodeSubject, importedServiceID string

	scenario.Precondition("operator starts node through terminal", func(t *testing.T) {
		start := terminal.run(t, context.Background(), "--output", "json", "node", "start")
		var response ardentsv1.CommandAckResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(start.stdout), &response), "stdout=%s", start.stdout)
		require.True(t, response.GetStatus().GetAccepted())
		require.Equal(t, "completed", response.GetStatus().GetState())
	})

	scenario.Step("terminal shows node, network and discovery readiness", func(t *testing.T) {
		status := terminal.run(t, context.Background(), "node", "status")
		require.Contains(t, status.stdout, "state: ready")
		require.Contains(t, status.stdout, "ready: true")

		network := terminal.run(t, context.Background(), "network", "status")
		require.Contains(t, network.stdout, "network status")
		require.Contains(t, network.stdout, "state: ready")
		require.Contains(t, network.stdout, "profile:")
		requireProtoJSONFields(t, terminal.run(t, context.Background(), "--output", "json", "network", "status"), "status", "network")

		discovery := terminal.run(t, context.Background(), "network", "discovery")
		require.Contains(t, discovery.stdout, "network discovery")
		require.Contains(t, discovery.stdout, "state: ready")
		requireProtoJSONFields(t, terminal.run(t, context.Background(), "--output", "json", "network", "discovery"), "status", "discovery")

		presence := terminal.run(t, context.Background(), "--output", "json", "network", "presence")
		var presenceResponse ardentsv1.LocalPresenceResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(presence.stdout), &presenceResponse), "stdout=%s", presence.stdout)
		require.True(t, presenceResponse.GetStatus().GetAccepted())
		require.NotEmpty(t, presenceResponse.GetPresence().GetState())
		presenceHuman := terminal.run(t, context.Background(), "network", "presence")
		require.Contains(t, presenceHuman.stdout, "network presence")
		require.Contains(t, presenceHuman.stdout, "state:")

		peers := terminal.run(t, context.Background(), "network", "peers")
		require.Contains(t, peers.stdout, "network peers")
		requireProtoJSONFields(t, terminal.run(t, context.Background(), "--output", "json", "network", "peers"), "status", "peers")

		recordList := terminal.run(t, context.Background(), "network", "records", "list")
		require.Contains(t, recordList.stdout, "network records")
	})

	scenario.Step("terminal imports records through CLI and classifies a stale record as rejected", func(t *testing.T) {
		beforeRoutes := terminal.run(t, context.Background(), "network", "routes", "--service", "echo")
		require.Contains(t, beforeRoutes.stdout, "outcome: not_found")

		records, err := remote.ListRecords()
		require.NoError(t, err)
		require.NotEmpty(t, records)
		for _, record := range records {
			record.Source = "bootstrap"
			if record.Node != nil {
				importedNodeSubject = record.Node.Principal
			}
			if record.Service != nil {
				importedServiceID = record.Service.ID
			}
			recordFile := writeProtoJSON(t, toProtoDiscoveryRecord(record))
			imported := terminal.run(t, context.Background(), "--output", "json", "network", "records", "import", "--file", recordFile)
			requireProtoJSONFields(t, imported, "status")
		}
		require.NotEmpty(t, importedNodeSubject)
		require.NotEmpty(t, importedServiceID)

		fresh, signer := terminalSignedNodeRecord(t, []string{"tcp://fresh"})
		freshFile := writeProtoJSON(t, toProtoDiscoveryRecord(fresh))
		events := captureTerminalNodeEvent(t, terminal, []string{"--output", "json", "--watch", "node", "events", "--limit", "1"}, func() {
			terminal.run(t, context.Background(), "network", "records", "import", "--file", freshFile)
		})
		var event map[string]any
		require.NoErrorf(t, json.Unmarshal(bytes.TrimSpace([]byte(events.stdout)), &event), "stdout=%s", events.stdout)
		require.Equal(t, "discovery", event["domain"])
		require.Equal(t, "imported", event["type"])

		humanFresh, _ := terminalSignedNodeRecord(t, []string{"tcp://human"})
		humanFreshFile := writeProtoJSON(t, toProtoDiscoveryRecord(humanFresh))
		humanEvents := captureTerminalNodeEvent(t, terminal, []string{"--watch", "node", "events", "--limit", "1"}, func() {
			terminal.run(t, context.Background(), "network", "records", "import", "--file", humanFreshFile)
		})
		require.Contains(t, humanEvents.stdout, "[discovery/imported]")

		stale := fresh
		staleNode := *fresh.Node
		staleNode.Endpoints = []string{"tcp://older"}
		stale.Node = &staleNode
		stale.IssuedAt = fresh.IssuedAt.Add(-time.Minute)
		signTerminalDiscoveryRecord(t, &stale, signer)
		staleFile := writeProtoJSON(t, toProtoDiscoveryRecord(stale))

		rejected := terminal.runOutcome(t, context.Background(), "--output", "json", "network", "records", "import", "--file", staleFile)
		require.Equal(t, 1, rejected.code)
		var rejectedResponse ardentsv1.RecordImportResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(rejected.stdout), &rejectedResponse), "stdout=%s", rejected.stdout)
		require.False(t, rejectedResponse.GetStatus().GetAccepted())
		var rejectedError map[string]any
		require.NoError(t, json.Unmarshal([]byte(rejected.stderr), &rejectedError))
		require.Contains(t, rejectedError["message"], "mutation response rejected")

		humanRejected := terminal.runOutcome(t, context.Background(), "network", "records", "import", "--file", staleFile)
		require.Equal(t, 1, humanRejected.code)
		require.Contains(t, humanRejected.stdout, "network records import")
		require.Contains(t, humanRejected.stdout, "status: rejected")
		require.Contains(t, humanRejected.stdout, "reason:")
		require.Contains(t, humanRejected.stdout, "accepted: false")
		require.Contains(t, humanRejected.stderr, "message: mutation response rejected")

	})

	scenario.Step("terminal resolves imported records and routes without domain shortcuts", func(t *testing.T) {
		testkit.WaitForCondition(t, 10*time.Second, "terminal network routes become usable after import", func() (bool, string) {
			routes := terminal.run(t, context.Background(), "network", "routes", "--service", "echo")
			if !bytes.Contains([]byte(routes.stdout), []byte("candidate:")) {
				return false, routes.stdout
			}
			return true, ""
		})

		afterDiscovery := terminal.run(t, context.Background(), "network", "discovery")
		require.Contains(t, afterDiscovery.stdout, "remote_records:")

		afterRoutes := terminal.run(t, context.Background(), "network", "routes", "--service", "echo")
		require.Contains(t, afterRoutes.stdout, "candidate:")
		require.NotContains(t, afterRoutes.stdout, "outcome: not_found")
		requireProtoJSONFields(t, terminal.run(t, context.Background(), "--output", "json", "network", "routes", "--service", "echo"), "status", "candidates", "route")

		listed := terminal.run(t, context.Background(), "--output", "json", "network", "records", "list")
		requireProtoJSONFields(t, listed, "status", "records")

		resolvedNode := terminal.run(t, context.Background(), "network", "resolve", "record", "--subject", importedNodeSubject, "--kind", "node")
		require.Contains(t, resolvedNode.stdout, "network resolve record")
		require.Contains(t, resolvedNode.stdout, "subject: "+importedNodeSubject)

		resolvedService := terminal.run(t, context.Background(), "--output", "json", "network", "resolve", "record", "--subject", importedServiceID, "--kind", "service")
		requireProtoJSONFields(t, resolvedService, "record", "outcome")

		service := terminal.run(t, context.Background(), "network", "resolve", "service", "--service", importedServiceID)
		require.Contains(t, service.stdout, "network resolve service")
		require.Contains(t, service.stdout, "service: "+importedServiceID)
		requireProtoJSONFields(t, terminal.run(t, context.Background(), "--output", "json", "network", "resolve", "service", "--service", importedServiceID), "service", "outcome", "matches", "route")

		response, err := http.Get(remoteEndpoint)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		require.Equal(t, http.StatusNoContent, response.StatusCode)
	})

	scenario.Assert("terminal shows shutdown truth without false ready state", func(t *testing.T) {
		stop := terminal.run(t, context.Background(), "--output", "json", "node", "stop")
		var response ardentsv1.CommandAckResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(stop.stdout), &response), "stdout=%s", stop.stdout)
		require.True(t, response.GetStatus().GetAccepted())
		require.Equal(t, "completed", response.GetStatus().GetState())

		status := terminal.run(t, context.Background(), "node", "status")
		require.Contains(t, status.stdout, "state: stopped")

		network := terminal.run(t, context.Background(), "network", "status")
		require.Contains(t, network.stdout, "state: stopped")
	})
}

func TestTerminalOperatorAdmissionRejectsSiblingAction(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "OCS-02-ADMISSION-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "ocs-02", "security"},
		Speed:       "fast",
		Environment: "local",
	})

	runtime := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-ocs02-admission",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	}).Runtime
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })
	fixture := testkit.NewOperatorCLIFixtureWithActions(t, runtime, catalogueActions(t,
		"node.start", "node.runtime", "network.status",
	))
	terminal := newTerminalHarnessFromFixture(t, fixture)

	scenario.Precondition("the exact granted Node and Network procedures succeed", func(t *testing.T) {
		terminal.run(t, context.Background(), "node", "start")
		status := terminal.run(t, context.Background(), "--output", "json", "network", "status")
		requireProtoJSONFields(t, status, "status", "network")
	})
	scenario.Assert("a sibling discovery action is denied without fallback", func(t *testing.T) {
		denied := terminal.runOutcome(t, context.Background(), "--output", "json", "network", "discovery")
		require.Equal(t, 1, denied.code)
		require.Empty(t, denied.stdout)
		var failure map[string]any
		require.NoError(t, json.Unmarshal([]byte(denied.stderr), &failure))
		require.Equal(t, "permission_denied", failure["code"])
	})
}

func TestTerminalWorkloadProcedureLifecycle(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "OCS-03-WORKLOAD-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "workload", "ocs-03"},
		Speed:       "default",
		Environment: "local",
	})

	runtime := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-ocs03-workload", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()},
		Privacy:   testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second)).Sender,
	}).Runtime
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })
	fixture := testkit.NewOperatorCLIFixtureWithActions(t, runtime, catalogueActions(t,
		// Every online CLI invocation performs this identity preflight before
		// dispatching its workload RPC.
		"node.start", "node.runtime",
		"workload.list", "workload.get", "workload.register", "workload.start", "workload.stop",
		"workload.restart", "workload.services", "workload.service", "workload.publication",
	))
	terminal := newTerminalHarnessFromFixture(t, fixture)

	servicePort := reserveTerminalPort(t)
	serviceEndpoint := terminalAdvertisedEndpoint(t, servicePort)
	serviceID := "svc.ocs03.echo"
	localServiceID := "svc.ocs03.local"
	workloadID := "work.ocs03.echo"
	specFile := writeProtoJSON(t, &ardentsv1.WorkloadSpecSnapshot{
		Id: workloadID, Kind: "service", Owner: "node", Config: terminalReadyConfig(t, servicePort), Desired: "present",
		Services: []*ardentsv1.PublishedServiceSnapshot{
			{
				Id: serviceID, Type: "echo", Owner: "node", Mode: "NetworkPublished",
				Endpoints:      []string{serviceEndpoint},
				ProbeEndpoints: []string{serviceEndpoint},
			},
			{
				Id: localServiceID, Type: "echo-local", Owner: "node", Mode: "LocalOnly",
				Endpoints:      []string{strings.TrimSuffix(serviceEndpoint, "/ready")},
				ProbeEndpoints: []string{serviceEndpoint},
			},
		},
	})
	idleFile := writeProtoJSON(t, &ardentsv1.WorkloadSpecSnapshot{
		Id: "work.ocs03.idle", Kind: "worker", Owner: "node",
		Config: terminalReadyConfig(t, reserveTerminalPort(t)), Desired: "present",
	})

	scenario.Precondition("operator starts the Node and registers workloads through exact CLI actions", func(t *testing.T) {
		terminal.run(t, context.Background(), "node", "start")
		registered := terminal.run(t, context.Background(), "workload", "register", "--file", specFile)
		require.Contains(t, registered.stdout, "workload register")
		require.Contains(t, registered.stdout, "workload: "+workloadID)

		idle := terminal.run(t, context.Background(), "--output", "json", "workload", "register", "--file", idleFile)
		var response ardentsv1.WorkloadCommandResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(idle.stdout), &response), "stdout=%s", idle.stdout)
		require.True(t, response.GetStatus().GetAccepted())
		require.Equal(t, "work.ocs03.idle", response.GetWorkload().GetSpec().GetId())
	})

	scenario.Step("inventory and exact workload queries preserve human and JSON truth", func(t *testing.T) {
		list := terminal.run(t, context.Background(), "workload", "list")
		require.Contains(t, list.stdout, "workload: "+workloadID)
		require.Contains(t, list.stdout, "workload: work.ocs03.idle")
		var listResponse ardentsv1.ListWorkloadsResponse
		listJSON := terminal.run(t, context.Background(), "--output", "json", "workload", "list")
		require.NoErrorf(t, protojson.Unmarshal([]byte(listJSON.stdout), &listResponse), "stdout=%s", listJSON.stdout)
		require.Len(t, listResponse.GetWorkloads(), 2)

		get := terminal.run(t, context.Background(), "workload", "get", workloadID)
		require.Contains(t, get.stdout, "workload get")
		require.Contains(t, get.stdout, "workload: "+workloadID)
		var getResponse ardentsv1.WorkloadStatusSnapshot
		getJSON := terminal.run(t, context.Background(), "--output", "json", "workload", "get", workloadID)
		require.NoErrorf(t, protojson.Unmarshal([]byte(getJSON.stdout), &getResponse), "stdout=%s", getJSON.stdout)
		require.Equal(t, workloadID, getResponse.GetSpec().GetId())
	})

	scenario.Step("CLI start exposes readiness separately from publication", func(t *testing.T) {
		started := terminal.run(t, context.Background(), "workload", "start", workloadID)
		require.Contains(t, started.stdout, "workload start")
		require.Contains(t, started.stdout, "accepted: true")

		service, publication := waitForTerminalServiceState(t, terminal, serviceID, true, true, "ready and published")
		require.True(t, service.GetReady())
		require.Equal(t, "local", service.GetRuntimeBacking())
		require.True(t, publication.GetPublished())

		services := terminal.run(t, context.Background(), "workload", "services")
		require.Contains(t, services.stdout, "service: "+serviceID)
		var servicesResponse ardentsv1.ListHostedServicesResponse
		servicesJSON := terminal.run(t, context.Background(), "--output", "json", "workload", "services")
		require.NoErrorf(t, protojson.Unmarshal([]byte(servicesJSON.stdout), &servicesResponse), "stdout=%s", servicesJSON.stdout)
		require.Len(t, servicesResponse.GetServices(), 2)

		serviceHuman := terminal.run(t, context.Background(), "workload", "service", serviceID)
		require.Contains(t, serviceHuman.stdout, "ready: true")
		publicationHuman := terminal.run(t, context.Background(), "workload", "publication", serviceID)
		require.Contains(t, publicationHuman.stdout, "published: true")

		localService, localPublication := waitForTerminalServiceState(t, terminal, localServiceID, true, false, "ready but unpublished")
		require.True(t, localService.GetReady())
		require.False(t, localService.GetPublished())
		require.False(t, localPublication.GetPublished())
		require.Contains(t, localPublication.GetReason(), "not network-published")

		localServiceHuman := terminal.run(t, context.Background(), "workload", "service", localServiceID)
		require.Contains(t, localServiceHuman.stdout, "ready: true")
		require.Contains(t, localServiceHuman.stdout, "published: false")
		localPublicationHuman := terminal.run(t, context.Background(), "workload", "publication", localServiceID)
		require.Contains(t, localPublicationHuman.stdout, "published: false")
		require.Contains(t, localPublicationHuman.stdout, "service mode is not network-published")
	})

	scenario.Step("restart and stop use CLI outcomes and withdraw publication", func(t *testing.T) {
		restarted := terminal.run(t, context.Background(), "--output", "json", "workload", "restart", workloadID)
		var restartResponse ardentsv1.WorkloadCommandResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(restarted.stdout), &restartResponse), "stdout=%s", restarted.stdout)
		require.True(t, restartResponse.GetStatus().GetAccepted())
		require.Equal(t, "running", restartResponse.GetWorkload().GetObserved())

		restartHuman := terminal.run(t, context.Background(), "workload", "restart", workloadID)
		require.Contains(t, restartHuman.stdout, "workload restart")
		require.Contains(t, restartHuman.stdout, "accepted: true")

		stopped := terminal.run(t, context.Background(), "--output", "json", "workload", "stop", workloadID)
		var stopResponse ardentsv1.WorkloadCommandResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(stopped.stdout), &stopResponse), "stdout=%s", stopped.stdout)
		require.True(t, stopResponse.GetStatus().GetAccepted())

		service, publication := waitForTerminalServiceState(t, terminal, serviceID, false, false, "unready and unpublished")
		require.False(t, service.GetReady())
		require.False(t, publication.GetPublished())

		started := terminal.run(t, context.Background(), "--output", "json", "workload", "start", workloadID)
		var startResponse ardentsv1.WorkloadCommandResponse
		require.NoErrorf(t, protojson.Unmarshal([]byte(started.stdout), &startResponse), "stdout=%s", started.stdout)
		require.True(t, startResponse.GetStatus().GetAccepted())
		terminal.run(t, context.Background(), "workload", "stop", workloadID)
	})

	scenario.Degraded("a non-accepted workload mutation is structured and nonzero", func(t *testing.T) {
		rejected := terminal.runOutcome(t, context.Background(), "--output", "json", "workload", "start", "work.missing")
		require.Equal(t, 1, rejected.code)
		require.Empty(t, rejected.stdout)
		var failure map[string]any
		require.NoError(t, json.Unmarshal([]byte(rejected.stderr), &failure))
		require.Equal(t, "not_found", failure["code"])
		require.Equal(t, "workload.start", failure["operation"])
	})
}

func TestTerminalContentRetentionAndTransferProcedure(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "OCS-04-DATA-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "data", "transfer", "ocs-04"},
		Speed:       "default",
		Environment: "local",
	})

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := []byte("0123456789abcdef0123456789abcdef")
	remoteHuman, err := sourceStore.StoreEncryptedBlob(
		appdata.Blob{MediaType: "application/octet-stream"},
		[]byte("ocs-04 human remote payload"),
		key,
		"",
	)
	require.NoError(t, err)
	remoteJSON, err := sourceStore.StoreEncryptedBlob(
		appdata.Blob{MediaType: "application/octet-stream"},
		[]byte("ocs-04 json remote payload"),
		key,
		"",
	)
	require.NoError(t, err)

	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	source := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-ocs04-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir}, Privacy: privacy.Sender,
	}).Node
	require.NoError(t, source.Start(context.Background()))
	t.Cleanup(func() { _ = source.Stop(context.Background()) })
	records, err := source.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.NotNil(t, records[0].Node)

	target := testkit.NewRuntime(t, runtimeinfra.Config{
		Name:  "terminal-ocs04-target",
		Boot:  runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Node.Endpoints...)},
		Trust: runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, source.Snapshot().Ident.PublicKey)},
		Data:  runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
	}).Runtime
	t.Cleanup(func() { _ = target.Stop(context.Background()) })
	fixture := testkit.NewOperatorCLIFixtureWithActions(t, target, catalogueActions(t,
		// node.start is the process precondition and node.runtime is the online
		// CLI identity preflight. Data authority is exact-resource only below.
		"node.start", "node.runtime",
	))
	terminal := newTerminalHarnessFromFixture(t, fixture)

	localPayload := []byte("ocs-04 local payload")
	_, localReference, err := contentpayload.DeriveIdentity(localPayload)
	require.NoError(t, err)
	var localBlob ardentsv1.BlobSnapshot
	const objectID = "obj.ocs04"
	const manifestID = "manifest.ocs04"
	fixture.GrantExact(t, catalogueActions(t, "data.inventory"), "content-inventory", "", false)
	fixture.GrantExact(t, catalogueActions(t, "data.objects.list"), "content-object-collection", "", false)
	fixture.GrantExact(t, catalogueActions(t, "data.objects.get", "data.objects.publish"), "content-object", objectID, true)
	fixture.GrantExact(t, catalogueActions(t, "data.blobs.list"), "content-blob-collection", "", false)
	fixture.GrantExact(t, catalogueActions(t,
		"data.blobs.get", "data.blobs.publish", "data.blobs.retain", "data.blobs.pin", "data.blobs.drop",
	), "content-blob", localReference.String(), true)
	fixture.GrantExact(t, catalogueActions(t, "data.blobs.fetch", "data.blobs.sources"), "content-blob", remoteHuman.Reference.String(), true)
	fixture.GrantExact(t, catalogueActions(t, "data.blobs.fetch"), "content-blob", remoteJSON.Reference.String(), true)
	fixture.GrantExact(t, catalogueActions(t, "data.manifests.list"), "content-manifest-collection", "", false)
	fixture.GrantExact(t, catalogueActions(t, "data.manifests.get", "data.manifests.publish"), "content-manifest", manifestID, true)
	fixture.GrantExact(t, catalogueActions(t, "data.transfers.list"), "transfer-collection", "", false)

	scenario.Precondition("operator starts the target and publishes file-backed Blob, Object and Manifest inputs", func(t *testing.T) {
		terminal.run(t, context.Background(), "node", "start")

		blobFile := writeProtoJSON(t, &ardentsv1.BlobSnapshot{
			MediaType: "text/plain",
			Payload:   localPayload,
		})
		blobJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "publish", "--file", blobFile)
		require.NoErrorf(t, protojson.Unmarshal([]byte(blobJSON.stdout), &localBlob), "stdout=%s", blobJSON.stdout)
		require.NotEmpty(t, localBlob.GetReference())
		require.Equal(t, "available-local", localBlob.GetState())
		blobHuman := terminal.run(t, context.Background(), "data", "blobs", "publish", "--file", blobFile)
		require.Contains(t, blobHuman.stdout, "data blob publish")
		require.Contains(t, blobHuman.stdout, "blob: "+localBlob.GetReference())

		objectFile := writeProtoJSON(t, &ardentsv1.ObjectSnapshot{
			Id: objectID, Type: "document",
			BlobRefs: []*ardentsv1.RefSnapshot{{Kind: "blob", Id: localBlob.GetReference()}},
		})
		objectHuman := terminal.run(t, context.Background(), "data", "objects", "publish", "--file", objectFile)
		require.Contains(t, objectHuman.stdout, "data object publish")
		require.Contains(t, objectHuman.stdout, "id: "+objectID)
		var object ardentsv1.ObjectSnapshot
		objectJSON := terminal.run(t, context.Background(), "--output", "json", "data", "objects", "publish", "--file", objectFile)
		require.NoErrorf(t, protojson.Unmarshal([]byte(objectJSON.stdout), &object), "stdout=%s", objectJSON.stdout)
		require.Equal(t, objectID, object.GetId())
		require.NotEmpty(t, object.GetOwner())

		manifestFile := writeProtoJSON(t, &ardentsv1.ManifestSnapshot{
			Id: manifestID, Kind: "blob-set",
			Refs: []*ardentsv1.RefSnapshot{{Kind: "blob", Id: localBlob.GetReference()}},
		})
		manifestHuman := terminal.run(t, context.Background(), "data", "manifests", "publish", "--file", manifestFile)
		require.Contains(t, manifestHuman.stdout, "data manifest publish")
		require.Contains(t, manifestHuman.stdout, "id: "+manifestID)
		var manifest ardentsv1.ManifestSnapshot
		manifestJSON := terminal.run(t, context.Background(), "--output", "json", "data", "manifests", "publish", "--file", manifestFile)
		require.NoErrorf(t, protojson.Unmarshal([]byte(manifestJSON.stdout), &manifest), "stdout=%s", manifestJSON.stdout)
		require.Equal(t, manifestID, manifest.GetId())
		require.Equal(t, object.GetOwner(), manifest.GetOwner())
	})

	scenario.Step("catalogue list/get and inventory preserve owner and content truth in human and JSON modes", func(t *testing.T) {
		objects := terminal.run(t, context.Background(), "data", "objects", "list")
		require.Contains(t, objects.stdout, "object: "+objectID)
		var objectList ardentsv1.ListObjectsResponse
		objectsJSON := terminal.run(t, context.Background(), "--output", "json", "data", "objects", "list")
		require.NoErrorf(t, protojson.Unmarshal([]byte(objectsJSON.stdout), &objectList), "stdout=%s", objectsJSON.stdout)
		require.Len(t, objectList.GetObjects(), 1)

		object := terminal.run(t, context.Background(), "data", "objects", "get", objectID)
		require.Contains(t, object.stdout, "id: "+objectID)
		var objectSnapshot ardentsv1.ObjectSnapshot
		objectJSON := terminal.run(t, context.Background(), "--output", "json", "data", "objects", "get", objectID)
		require.NoErrorf(t, protojson.Unmarshal([]byte(objectJSON.stdout), &objectSnapshot), "stdout=%s", objectJSON.stdout)
		require.Equal(t, objectID, objectSnapshot.GetId())

		blobs := terminal.run(t, context.Background(), "data", "blobs", "list")
		require.Contains(t, blobs.stdout, "blob: "+localBlob.GetReference())
		var blobList ardentsv1.ListBlobsResponse
		blobsJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "list")
		require.NoErrorf(t, protojson.Unmarshal([]byte(blobsJSON.stdout), &blobList), "stdout=%s", blobsJSON.stdout)
		require.NotEmpty(t, blobList.GetBlobs())

		blob := terminal.run(t, context.Background(), "data", "blobs", "get", localBlob.GetReference())
		require.Contains(t, blob.stdout, "state: available-local")
		var blobSnapshot ardentsv1.BlobSnapshot
		blobJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "get", localBlob.GetReference())
		require.NoErrorf(t, protojson.Unmarshal([]byte(blobJSON.stdout), &blobSnapshot), "stdout=%s", blobJSON.stdout)
		require.Equal(t, localBlob.GetReference(), blobSnapshot.GetReference())

		manifests := terminal.run(t, context.Background(), "data", "manifests", "list")
		require.Contains(t, manifests.stdout, "manifest: "+manifestID)
		var manifestList ardentsv1.ListManifestsResponse
		manifestsJSON := terminal.run(t, context.Background(), "--output", "json", "data", "manifests", "list")
		require.NoErrorf(t, protojson.Unmarshal([]byte(manifestsJSON.stdout), &manifestList), "stdout=%s", manifestsJSON.stdout)
		require.Len(t, manifestList.GetManifests(), 1)

		manifest := terminal.run(t, context.Background(), "data", "manifests", "get", manifestID)
		require.Contains(t, manifest.stdout, "id: "+manifestID)
		var manifestSnapshot ardentsv1.ManifestSnapshot
		manifestJSON := terminal.run(t, context.Background(), "--output", "json", "data", "manifests", "get", manifestID)
		require.NoErrorf(t, protojson.Unmarshal([]byte(manifestJSON.stdout), &manifestSnapshot), "stdout=%s", manifestJSON.stdout)
		require.Equal(t, manifestID, manifestSnapshot.GetId())

		inventory := terminal.run(t, context.Background(), "data", "inventory")
		require.Contains(t, inventory.stdout, "objects: 1")
		require.Contains(t, inventory.stdout, "manifests: 1")
		var inventorySnapshot ardentsv1.DataInventorySnapshot
		inventoryJSON := terminal.run(t, context.Background(), "--output", "json", "data", "inventory")
		require.NoErrorf(t, protojson.Unmarshal([]byte(inventoryJSON.stdout), &inventorySnapshot), "stdout=%s", inventoryJSON.stdout)
		require.EqualValues(t, 1, inventorySnapshot.GetObjects())
		require.EqualValues(t, 1, inventorySnapshot.GetManifests())
	})

	scenario.Step("retention mutations expose exact state without changing the protobuf JSON family", func(t *testing.T) {
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second).Format(time.RFC3339)
		retained := terminal.run(t, context.Background(), "data", "blobs", "retain", "--id", localBlob.GetReference(), "--expires-at", expiresAt)
		require.Contains(t, retained.stdout, "retention: temporary")
		var retainedSnapshot ardentsv1.BlobSnapshot
		retainedJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "retain", "--id", localBlob.GetReference(), "--expires-at", expiresAt)
		require.NoErrorf(t, protojson.Unmarshal([]byte(retainedJSON.stdout), &retainedSnapshot), "stdout=%s", retainedJSON.stdout)
		require.Equal(t, "retained-temporary", retainedSnapshot.GetState())

		pinned := terminal.run(t, context.Background(), "data", "blobs", "pin", localBlob.GetReference())
		require.Contains(t, pinned.stdout, "retention: pinned")
		var pinnedSnapshot ardentsv1.BlobSnapshot
		pinnedJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "pin", localBlob.GetReference())
		require.NoErrorf(t, protojson.Unmarshal([]byte(pinnedJSON.stdout), &pinnedSnapshot), "stdout=%s", pinnedJSON.stdout)
		require.Equal(t, "pinned", pinnedSnapshot.GetState())

		dropped := terminal.run(t, context.Background(), "data", "blobs", "drop", localBlob.GetReference())
		require.Contains(t, dropped.stdout, "state: deleted")
		blobFile := writeProtoJSON(t, &ardentsv1.BlobSnapshot{MediaType: "text/plain", Payload: localPayload})
		terminal.run(t, context.Background(), "data", "blobs", "publish", "--file", blobFile)
		var droppedSnapshot ardentsv1.BlobSnapshot
		droppedJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "drop", localBlob.GetReference())
		require.NoErrorf(t, protojson.Unmarshal([]byte(droppedJSON.stdout), &droppedSnapshot), "stdout=%s", droppedJSON.stdout)
		require.Equal(t, "deleted", droppedSnapshot.GetState())
	})

	scenario.Step("private fetch keeps Blob success distinct from transfer completion and exposes progress queries", func(t *testing.T) {
		fetchHuman := terminal.run(t, context.Background(), "data", "blobs", "fetch", remoteHuman.Reference.String())
		require.Contains(t, fetchHuman.stdout, "data blob fetch")
		require.Contains(t, fetchHuman.stdout, "state: available-local")
		require.NotContains(t, fetchHuman.stdout, "completed")

		fetchJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "fetch", remoteJSON.Reference.String())
		var fetched ardentsv1.BlobSnapshot
		require.NoErrorf(t, protojson.Unmarshal([]byte(fetchJSON.stdout), &fetched), "stdout=%s", fetchJSON.stdout)
		require.Equal(t, "available-local", fetched.GetState())
		require.Equal(t, remoteJSON.Reference.String(), fetched.GetReference())

		sources := terminal.run(t, context.Background(), "data", "blobs", "sources", remoteHuman.Reference.String())
		require.Contains(t, sources.stdout, "data blob sources")
		require.Contains(t, sources.stdout, "usable: true")
		var sourceList ardentsv1.ListBlobSourcesResponse
		sourcesJSON := terminal.run(t, context.Background(), "--output", "json", "data", "blobs", "sources", remoteHuman.Reference.String())
		require.NoErrorf(t, protojson.Unmarshal([]byte(sourcesJSON.stdout), &sourceList), "stdout=%s", sourcesJSON.stdout)
		require.NotEmpty(t, sourceList.GetSources())

		transfers := terminal.run(t, context.Background(), "data", "transfers", "list")
		require.Contains(t, transfers.stdout, "state: completed")
		var transferList ardentsv1.ListTransfersResponse
		transfersJSON := terminal.run(t, context.Background(), "--output", "json", "data", "transfers", "list")
		require.NoErrorf(t, protojson.Unmarshal([]byte(transfersJSON.stdout), &transferList), "stdout=%s", transfersJSON.stdout)
		transfer := findTransferByResource(t, transferList.GetTransfers(), remoteHuman.Reference.String())
		require.Equal(t, "completed", transfer.GetState())
		fixture.GrantExact(t, catalogueActions(t, "data.transfers.get"), "transfer", transfer.GetId(), false)

		transferHuman := terminal.run(t, context.Background(), "data", "transfers", "get", transfer.GetId())
		require.Contains(t, transferHuman.stdout, "transfer: "+transfer.GetId())
		require.Contains(t, transferHuman.stdout, "state: completed")
		var transferResponse ardentsv1.GetTransferResponse
		transferJSON := terminal.run(t, context.Background(), "--output", "json", "data", "transfers", "get", transfer.GetId())
		require.NoErrorf(t, protojson.Unmarshal([]byte(transferJSON.stdout), &transferResponse), "stdout=%s", transferJSON.stdout)
		require.Equal(t, transfer.GetId(), transferResponse.GetTransfer().GetId())
	})

	scenario.Degraded("an ungranted sibling resource fails closed before content lookup", func(t *testing.T) {
		missing := terminal.runOutcome(t, context.Background(), "--output", "json", "data", "objects", "get", "obj.missing")
		require.Equal(t, 1, missing.code)
		require.Empty(t, missing.stdout)
		var failure map[string]any
		require.NoError(t, json.Unmarshal([]byte(missing.stderr), &failure))
		require.Equal(t, "permission_denied", failure["code"])
	})
}

func TestTerminalPrincipalAccessAdministrationProcedure(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "OCS-05-IDENTITY-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "identity", "session", "ocs-05"},
		Speed:       "default",
		Environment: "local",
	})

	runtime := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-ocs05-identity",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	}).Runtime
	t.Cleanup(func() { _ = runtime.Stop(context.Background()) })
	fixture := testkit.NewOperatorCLIFixtureWithActions(t, runtime, catalogueActions(t,
		// node.runtime is required only by the persistent shell preflight.
		"node.runtime",
		"identity.enroll", "identity.grant.list", "identity.grant.issue",
		"identity.grant.revoke", "identity.device.revoke",
	))
	terminal := newTerminalHarnessFromFixture(t, fixture)

	identityDir := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(identityDir))
	laterRoot := filepath.Join(identityDir, "later-root.json")
	laterDevice := filepath.Join(identityDir, "later-device.json")
	appRoot := filepath.Join(identityDir, "application-root.json")
	var laterPrincipal, laterDeviceID, applicationPrincipal string

	scenario.Precondition("offline custody creates public enrollment material without leaking protected paths", func(t *testing.T) {
		principal := terminal.run(t, context.Background(), "--output", "json", "identity", "principal", "create", "--signer-file", laterRoot)
		var principalView map[string]any
		require.NoError(t, json.Unmarshal([]byte(principal.stdout), &principalView))
		laterPrincipal, _ = principalView["principal"].(string)
		require.NotEmpty(t, laterPrincipal)
		require.NotContains(t, principal.stdout, laterRoot)

		device := terminal.run(t, context.Background(), "--output", "json", "identity", "device", "create",
			"--root-signer-file", laterRoot, "--signer-file", laterDevice, "--valid-for", "1h")
		var deviceView map[string]any
		require.NoError(t, json.Unmarshal([]byte(device.stdout), &deviceView))
		laterDeviceID, _ = deviceView["device_id"].(string)
		require.NotEmpty(t, laterDeviceID)
		require.NotContains(t, device.stdout, laterRoot)
		require.NotContains(t, device.stdout, laterDevice)

		app := terminal.run(t, context.Background(), "--output", "json", "identity", "principal", "create", "--signer-file", appRoot)
		var appView map[string]any
		require.NoError(t, json.Unmarshal([]byte(app.stdout), &appView))
		applicationPrincipal, _ = appView["principal"].(string)
		require.NotEmpty(t, applicationPrincipal)
	})

	scenario.Step("later Principal enrollment and grant administration preserve explicit request IDs", func(t *testing.T) {
		const enrollmentRequest = "ocs05-enroll-001"
		enrolled := terminal.run(t, context.Background(), "--output", "json", "identity", "enroll",
			"--root-signer-file", laterRoot, "--device-signer-file", laterDevice, "--request-id", enrollmentRequest)
		var enrollment map[string]any
		require.NoError(t, json.Unmarshal([]byte(enrolled.stdout), &enrollment))
		require.Equal(t, laterPrincipal, enrollment["principal"])
		require.Equal(t, "administrator", enrollment["mode"])
		require.Equal(t, enrollmentRequest, enrollment["request_id"])
		require.NotContains(t, enrolled.stdout, laterRoot)
		require.NotContains(t, enrolled.stdout, laterDevice)

		const issueRequest = "ocs05-grant-issue-001"
		issued := terminal.run(t, context.Background(), "--output", "json", "identity", "grant", "issue",
			"--subject", laterPrincipal, "--action", "node.status", "--scope", "node",
			"--valid-for", "1h", "--request-id", issueRequest)
		var issue map[string]any
		require.NoError(t, json.Unmarshal([]byte(issued.stdout), &issue))
		require.Equal(t, "grant_issue", issue["operation"])
		require.Equal(t, issueRequest, issue["request_id"])
		grantID, _ := issue["result_id"].(string)
		require.NotEmpty(t, grantID)

		list := terminal.run(t, context.Background(), "identity", "grant", "list", "--subject", laterPrincipal)
		require.Contains(t, list.stdout, "id: "+grantID)
		require.Contains(t, list.stdout, "actions: node.status")
		listJSON := terminal.run(t, context.Background(), "--output", "json", "identity", "grant", "list", "--subject", laterPrincipal)
		var grants struct {
			Grants []struct {
				ID string `json:"id"`
			} `json:"grants"`
		}
		require.NoError(t, json.Unmarshal([]byte(listJSON.stdout), &grants))
		require.NotEmpty(t, grants.Grants)

		const revokeRequest = "ocs05-grant-revoke-001"
		revoked := terminal.run(t, context.Background(), "identity", "grant", "revoke",
			"--subject", laterPrincipal, "--grant-id", grantID, "--request-id", revokeRequest, "--yes")
		require.Contains(t, revoked.stdout, "request_id: "+revokeRequest)
		require.Contains(t, revoked.stdout, "result_id: ar1_")

		afterRevoke := terminal.run(t, context.Background(), "--output", "json", "identity", "grant", "list", "--subject", laterPrincipal)
		var reconciled struct {
			Grants []struct {
				ID      string `json:"id"`
				Revoked bool   `json:"revoked"`
			} `json:"grants"`
		}
		require.NoError(t, json.Unmarshal([]byte(afterRevoke.stdout), &reconciled))
		var foundRevoked bool
		for _, grant := range reconciled.Grants {
			if grant.ID == grantID {
				foundRevoked = grant.Revoked
			}
		}
		require.True(t, foundRevoked, "revoked grant %s must remain reconcilable in grant list", grantID)
	})

	scenario.Step("Application ticket and Delegation revocation keep protected artifacts out of terminal output", func(t *testing.T) {
		ticketPath := filepath.Join(identityDir, "application-ticket")
		ticket := terminal.run(t, context.Background(), "--output", "json", "identity", "application-ticket", "issue",
			"--principal", applicationPrincipal, "--action", "application.content.get", "--out-file", ticketPath)
		require.FileExists(t, ticketPath)
		ticketSecret, err := os.ReadFile(ticketPath)
		require.NoError(t, err)
		require.NotEmpty(t, ticketSecret)
		require.NotContains(t, ticket.stdout, string(ticketSecret))
		require.NotContains(t, ticket.stderr, string(ticketSecret))
		var ticketView map[string]any
		require.NoError(t, json.Unmarshal([]byte(ticket.stdout), &ticketView))
		require.Equal(t, "application_enrollment_ticket_issue", ticketView["operation"])
		require.Equal(t, ticketPath, ticketView["protected_output"])

		delegationPath := filepath.Join(identityDir, "delegation")
		revocationPath := filepath.Join(identityDir, "delegation-revocation")
		delegation := terminal.run(t, context.Background(), "--output", "json", "identity", "delegation", "issue",
			"--application", applicationPrincipal, "--action", "application.content.get",
			"--scope", "principal-owned", "--out-file", delegationPath, "--signer-file", fixture.SignerFile, "--yes")
		delegationRaw, err := os.ReadFile(delegationPath)
		require.NoError(t, err)
		require.NotContains(t, delegation.stdout, string(delegationRaw))
		require.NotContains(t, delegation.stdout, fixture.SignerFile)

		revocation := terminal.run(t, context.Background(), "--output", "json", "identity", "delegation", "revoke",
			"--delegation-file", delegationPath, "--out-file", revocationPath, "--signer-file", fixture.SignerFile, "--yes")
		revocationRaw, err := os.ReadFile(revocationPath)
		require.NoError(t, err)
		require.NotContains(t, revocation.stdout, string(revocationRaw))
		require.NotContains(t, revocation.stdout, delegationPath)
		require.NotContains(t, revocation.stdout, fixture.SignerFile)

		imported := terminal.run(t, context.Background(), "--output", "json", "identity", "delegation", "import-revocation",
			"--revocation-file", revocationPath)
		require.NotContains(t, imported.stdout, string(revocationRaw))
		require.NotContains(t, imported.stdout, revocationPath)
		var importedView map[string]any
		require.NoError(t, json.Unmarshal([]byte(imported.stdout), &importedView))
		require.Equal(t, "delegation_revocation_import", importedView["operation"])
		require.NotEmpty(t, importedView["revocation_id"])
	})

	scenario.Step("device revocation is reconcilable and Session commands retain one-shot and live-shell truth", func(t *testing.T) {
		const deviceRequest = "ocs05-device-revoke-001"
		revoked := terminal.run(t, context.Background(), "--output", "json", "identity", "device", "revoke",
			"--principal", laterPrincipal, "--device-id", laterDeviceID, "--request-id", deviceRequest)
		var deviceMutation map[string]any
		require.NoError(t, json.Unmarshal([]byte(revoked.stdout), &deviceMutation))
		require.Equal(t, "device_revoke", deviceMutation["operation"])
		require.Equal(t, deviceRequest, deviceMutation["request_id"])
		require.NotEmpty(t, deviceMutation["result_id"])

		revokedLogin := terminal.runOutcome(t, context.Background(), "--signer-file", laterDevice, "--output", "json", "identity", "login")
		require.Equal(t, 1, revokedLogin.code)
		require.Empty(t, revokedLogin.stdout)
		var revokedFailure map[string]any
		require.NoError(t, json.Unmarshal([]byte(revokedLogin.stderr), &revokedFailure))
		require.Equal(t, "unauthenticated", revokedFailure["code"])
		require.NotContains(t, revokedLogin.stderr, laterDevice)

		login := terminal.run(t, context.Background(), "--output", "json", "identity", "login")
		var loginView map[string]any
		require.NoError(t, json.Unmarshal([]byte(login.stdout), &loginView))
		require.Equal(t, "authenticated", loginView["status"])
		require.NotContains(t, login.stdout, "session_secret")
		status := terminal.run(t, context.Background(), "--output", "json", "identity", "status")
		require.JSONEq(t, `{"status":"not_authenticated"}`, status.stdout)
		logout := terminal.run(t, context.Background(), "--output", "json", "identity", "logout")
		require.JSONEq(t, `{"status":"not_authenticated"}`, logout.stdout)

		shell := terminal.runInput(t, context.Background(), strings.Join([]string{
			"identity login",
			"identity status",
			"identity logout",
			"identity status",
			"exit",
			"",
		}, "\n"), "shell")
		require.Equalf(t, 0, shell.code, "stderr=%s", shell.stderr)
		require.Empty(t, shell.stderr)
		require.Contains(t, shell.stdout, "status: authenticated")
		require.Contains(t, shell.stdout, "status: not_authenticated")
		require.NotContains(t, shell.stdout, "session_secret")
		require.NotContains(t, shell.stdout, fixture.SignerFile)
	})

	scenario.Degraded("an invalid administrative target retains its request ID in structured failure output", func(t *testing.T) {
		const requestID = "ocs05-invalid-target-001"
		rejected := terminal.runOutcome(t, context.Background(), "--output", "json", "identity", "grant", "issue",
			"--subject", applicationPrincipal, "--action", "node.start", "--scope", "node",
			"--request-id", requestID)
		require.Equal(t, 1, rejected.code)
		require.Empty(t, rejected.stdout)
		var failure map[string]any
		require.NoError(t, json.Unmarshal([]byte(rejected.stderr), &failure))
		require.Equal(t, "invalid_argument", failure["code"])
		require.Contains(t, failure["message"], requestID)
	})
}

func TestTerminalServiceAndDataSurfaceReadiness(t *testing.T) {
	testkit.ConfigureLoopbackTransport(t)
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "network-operator-terminal",
		ScenarioID:  "E2E-SVC-DATA-SURFACE-001",
		Suite:       "e2e",
		Tags:        []string{"e2e", "network-operator-terminal", "workload", "data", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := []byte("0123456789abcdef0123456789abcdef")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoError(t, err)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))

	source := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-service-data-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir}, Privacy: privacy.Sender,
	}).Node
	require.NoError(t, source.Start(context.Background()))
	t.Cleanup(func() { _ = source.Stop(context.Background()) })
	records, err := source.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	require.NotNil(t, records[0].Node)

	sut := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "terminal-service-data-sut", NodeProfile: transport.NodeProfileServiceNode,
		Boot:      runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Node.Endpoints...)},
		Trust:     runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, source.Snapshot().Ident.PublicKey)},
		Transport: runtimeinfra.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: transport.ReachabilityPrivateLAN},
		Data:      runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Receiver,
		DiscoveryRefreshInterval: 500 * time.Millisecond,
	}).Runtime
	t.Cleanup(func() { _ = sut.Stop(context.Background()) })
	terminal := newTerminalHarness(t, sut)

	servicePort := reserveTerminalPort(t)
	serviceEndpoint := terminalAdvertisedEndpoint(t, servicePort)
	specFile := writeProtoJSON(t, &ardentsv1.WorkloadSpecSnapshot{
		Id:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  terminalReadyConfig(t, servicePort),
		Desired: "present",
		Services: []*ardentsv1.PublishedServiceSnapshot{{
			Id:             "svc.work.echo",
			Type:           "echo",
			Owner:          "node",
			Mode:           "NetworkPublished",
			Endpoints:      []string{serviceEndpoint},
			ProbeEndpoints: []string{serviceEndpoint},
		}},
	})

	scenario.Precondition("operator starts node and registers workload through terminal", func(t *testing.T) {
		start := terminal.run(t, context.Background(), "node", "start")
		require.Contains(t, start.stdout, "node start")
		require.Contains(t, start.stdout, "status: completed")

		register := terminal.run(t, context.Background(), "workload", "register", "--file", specFile)
		require.Contains(t, register.stdout, "workload register")
		require.Contains(t, register.stdout, "workload: work.echo")
	})

	scenario.Step("terminal shows workload publication truth after start", func(t *testing.T) {
		require.NoError(t, testkit.Workloads(sut).Start(context.Background(), "work.echo"))

		testkit.WaitForCondition(t, 10*time.Second, "terminal publication becomes visible", func() (bool, string) {
			publication := terminal.run(t, context.Background(), "workload", "publication", "svc.work.echo")
			if !bytes.Contains([]byte(publication.stdout), []byte("published: true")) {
				return false, publication.stdout
			}
			return true, ""
		})

		service := terminal.run(t, context.Background(), "workload", "service", "svc.work.echo")
		require.Contains(t, service.stdout, "workload service")
		require.Contains(t, service.stdout, "published: true")
		response, err := http.Get(serviceEndpoint)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		require.Equal(t, http.StatusNoContent, response.StatusCode)
	})

	scenario.Step("terminal executes data fetch flow and exposes transfer truth", func(t *testing.T) {
		fetch := terminal.run(t, context.Background(), "data", "blobs", "fetch", stored.Reference.String())
		require.Contains(t, fetch.stdout, "data blob fetch")
		require.Contains(t, fetch.stdout, "blob: "+stored.Reference.String())

		testkit.WaitForCondition(t, 10*time.Second, "terminal transfer becomes completed", func() (bool, string) {
			list := terminal.run(t, context.Background(), "data", "transfers", "list")
			if !bytes.Contains([]byte(list.stdout), []byte("  state: completed")) {
				return false, list.stdout
			}
			return true, ""
		})

		transfers, err := terminal.service().ListTransfers(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ListTransfersRequest{}))
		require.NoError(t, err)
		transfer := findTransferByResource(t, transfers.Msg.GetTransfers(), stored.Reference.String())

		getTransfer := terminal.run(t, context.Background(), "data", "transfers", "get", transfer.GetId())
		require.Contains(t, getTransfer.stdout, "data transfer")
		require.Contains(t, getTransfer.stdout, "  state: completed")

		inventory := terminal.run(t, context.Background(), "data", "inventory")
		require.Contains(t, inventory.stdout, "data inventory")
		require.Contains(t, inventory.stdout, "local_blobs:")
	})

	scenario.Degraded("terminal shows publication withdrawal and diagnostics explanation after workload stop", func(t *testing.T) {
		require.NoError(t, testkit.Workloads(sut).Stop(context.Background(), "work.echo"))

		testkit.WaitForCondition(t, 10*time.Second, "terminal publication is withdrawn", func() (bool, string) {
			publication := terminal.run(t, context.Background(), "workload", "publication", "svc.work.echo")
			if !bytes.Contains([]byte(publication.stdout), []byte("published: false")) {
				return false, publication.stdout
			}
			return true, ""
		})

		publication := terminal.run(t, context.Background(), "workload", "publication", "svc.work.echo")
		require.Contains(t, publication.stdout, "published: false")

		explain := terminal.run(t, context.Background(), "diagnostics", "explain", "--scope", "service", "--resource-id", "svc.work.echo")
		require.Contains(t, explain.stdout, "diagnostics explain")
		require.Contains(t, explain.stdout, "resource_id: svc.work.echo")
		require.Contains(t, explain.stdout, "state: degraded")
		require.Contains(t, explain.stdout, "reason: runtime_inactive")
		require.Contains(t, explain.stdout, "impact: service is not discoverable")
	})
}

type terminalHarness struct {
	nodePrincipal string
	client        cliclient.Service
	authPacer     *terminalAuthPacer
}

type terminalAuthPacer struct {
	mu                 sync.Mutex
	begins             int
	nextAllowed        time.Time
	persistentReserved bool
}

type terminalResult struct {
	stdout string
	stderr string
	code   int
}

func newTerminalHarness(t *testing.T, runtime *runtimeprocess.Node) terminalHarness {
	t.Helper()
	fixture := testkit.NewOperatorCLIFixture(t, runtime)
	return newTerminalHarnessFromFixture(t, fixture)
}

func newTerminalHarnessFromFixture(t *testing.T, fixture testkit.OperatorCLIFixture) terminalHarness {
	t.Helper()
	t.Setenv("ARDENTS_ADDR", fixture.Addr)
	t.Setenv("ARDENTS_SIGNER_FILE", fixture.SignerFile)
	t.Setenv("ARDENTS_EXPECTED_PRINCIPAL", fixture.NodePrincipal)
	return terminalHarness{
		nodePrincipal: fixture.NodePrincipal, client: fixture.Client,
		// Fixture provisioning performs enrollment-proof and session Begin
		// calls against the same in-process Unix source before CLI commands.
		authPacer: &terminalAuthPacer{begins: 2},
	}
}

func (h terminalHarness) run(t *testing.T, ctx context.Context, args ...string) terminalResult {
	t.Helper()
	result := h.runOutcome(t, ctx, args...)
	require.Equalf(t, 0, result.code, "stderr=%s", result.stderr)
	require.Empty(t, result.stderr)
	return result
}

func (h terminalHarness) runOutcome(t *testing.T, ctx context.Context, args ...string) terminalResult {
	t.Helper()
	h.authPacer.wait()
	return h.runOutcomeUnpaced(ctx, args...)
}

func (h terminalHarness) runOutcomeUnpaced(ctx context.Context, args ...string) terminalResult {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(ctx, args, &stdout, &stderr)
	return terminalResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func (h terminalHarness) runInput(t *testing.T, ctx context.Context, input string, args ...string) terminalResult {
	t.Helper()
	h.authPacer.wait()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.RunWithIOForTest(ctx, args, strings.NewReader(input), &stdout, &stderr)
	return terminalResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func captureTerminalNodeEvent(t *testing.T, terminal terminalHarness, args []string, trigger func()) terminalResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ready := make(chan struct{})
	result := make(chan terminalResult, 1)
	go func() {
		terminal.authPacer.wait()
		close(ready)
		result <- terminal.runOutcomeUnpaced(ctx, args...)
	}()
	<-ready
	time.Sleep(time.Second)
	trigger()
	select {
	case event := <-result:
		require.Equalf(t, 0, event.code, "stderr=%s", event.stderr)
		require.Empty(t, event.stderr)
		return event
	case <-ctx.Done():
		require.FailNow(t, "terminal event stream did not finish", "%v", ctx.Err())
		return terminalResult{}
	}
}

func (h terminalHarness) service() cliclient.Service {
	if h.authPacer != nil && h.authPacer.reservePersistent() {
		h.authPacer.wait()
	}
	return h.client
}

func (p *terminalAuthPacer) reservePersistent() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.persistentReserved {
		return false
	}
	p.persistentReserved = true
	return true
}

func (p *terminalAuthPacer) wait() {
	if p == nil {
		return
	}
	const refillSlack = 100 * time.Millisecond
	interval := time.Minute / time.Duration(identitycontract.BeginRatePerMinute)

	p.mu.Lock()
	now := time.Now()
	if p.begins < identitycontract.BeginRateBurst {
		p.begins++
		if p.begins == identitycontract.BeginRateBurst {
			p.nextAllowed = now.Add(interval + refillSlack)
		}
		p.mu.Unlock()
		return
	}
	target := p.nextAllowed
	if target.Before(now) {
		target = now
	}
	p.nextAllowed = target.Add(interval + refillSlack)
	p.mu.Unlock()

	if delay := time.Until(target); delay > 0 {
		time.Sleep(delay)
	}
}

func writeProtoJSON(t *testing.T, msg proto.Message) string {
	t.Helper()

	data, err := protojson.Marshal(msg)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "input.json")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func requireProtoJSONFields(t *testing.T, result terminalResult, fields ...string) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoErrorf(t, json.Unmarshal([]byte(result.stdout), &decoded), "stdout=%s", result.stdout)
	for _, field := range fields {
		require.Contains(t, decoded, field)
	}
	return decoded
}

func waitForTerminalServiceState(
	t *testing.T,
	terminal terminalHarness,
	serviceID string,
	expectedReady bool,
	expectedPublished bool,
	description string,
) (*ardentsv1.HostedServiceStatusSnapshot, *ardentsv1.PublicationStatusSnapshot) {
	t.Helper()
	var service ardentsv1.GetHostedServiceResponse
	var publication ardentsv1.ServicePublicationStatusResponse
	testkit.WaitForCondition(t, 45*time.Second, "terminal service becomes "+description, func() (bool, string) {
		serviceResult := terminal.run(t, context.Background(), "--output", "json", "workload", "service", serviceID)
		require.NoError(t, protojson.Unmarshal([]byte(serviceResult.stdout), &service))
		publicationResult := terminal.run(t, context.Background(), "--output", "json", "workload", "publication", serviceID)
		require.NoError(t, protojson.Unmarshal([]byte(publicationResult.stdout), &publication))
		currentService, currentPublication := service.GetService(), publication.GetPublication()
		return currentService.GetReady() == expectedReady && currentPublication.GetPublished() == expectedPublished,
			fmt.Sprintf("ready=%t backing=%q published=%t", currentService.GetReady(), currentService.GetRuntimeBacking(), currentPublication.GetPublished())
	})
	return service.GetService(), publication.GetPublication()
}

func catalogueActions(t *testing.T, ids ...string) []identityaccess.Action {
	t.Helper()
	actions := make([]identityaccess.Action, len(ids))
	for index, id := range ids {
		var spec catalog.CommandSpec
		var ok bool
		for _, candidate := range catalog.Commands() {
			if candidate.ID == id {
				spec, ok = candidate, true
				break
			}
		}
		require.Truef(t, ok, "unknown command catalogue ID %q", id)
		require.NotEmptyf(t, spec.Action, "command %q has no protected action", id)
		actions[index] = identityaccess.Action(spec.Action)
	}
	return actions
}

func terminalReadyConfig(t *testing.T, port int) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestTerminalReadyHelper"},
		"env": map[string]string{"ARDENTS_TERMINAL_READY_HELPER": "1", "ARDENTS_TERMINAL_READY_ADDRESS": fmt.Sprintf("%s:%d", terminalPrivateIPv4(t), port)}})
	require.NoError(t, err)
	return string(raw)
}

func reserveTerminalPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	return listener.Addr().(*net.TCPAddr).Port
}

//goland:noinspection ALL
func terminalAdvertisedEndpoint(t *testing.T, port int) string {
	t.Helper()
	return fmt.Sprintf("http://%s:%d/ready", terminalPrivateIPv4(t), port)
}

func terminalPrivateIPv4(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("private-interface service reachability is an e2e Linux acceptance scenario")
	}
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return ip.String()
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return ""
}

func toProtoDiscoveryRecord(record discoveryapi.CatalogRecordSnapshot) *ardentsv1.DiscoveryRecord {
	out := &ardentsv1.DiscoveryRecord{
		Version:     record.Version,
		IssuedAtV1:  timestampProto(record.IssuedAt),
		ExpiresAtV1: timestampProto(record.ExpiresAt),
		SignatureV1: record.Signature,
		SourceV1:    record.Source,
	}
	if record.Node != nil {
		out.Facts = &ardentsv1.DiscoveryRecord_NodeFacts{NodeFacts: &ardentsv1.NodeDiscoveryFacts{
			Principal: record.Node.Principal,
			PublicKey: record.Node.PublicKey,
			Endpoints: append([]string(nil), record.Node.Endpoints...),
		}}
	}
	if record.Service != nil {
		out.Facts = &ardentsv1.DiscoveryRecord_ServiceFacts{ServiceFacts: &ardentsv1.ServiceDiscoveryFacts{
			ServiceId:     record.Service.ID,
			ServiceType:   record.Service.Type,
			NodePrincipal: record.Service.NodePrincipal,
			WorkloadId:    record.Service.WorkloadID,
			Mode:          record.Service.Mode,
			PublicKey:     record.Service.PublicKey,
			Endpoints:     append([]string(nil), record.Service.Endpoints...),
		}}
	}
	return out
}

func terminalSignedNodeRecord(t *testing.T, endpoints []string) (discoveryapi.CatalogRecordSnapshot, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityprincipal.FromPublicKey(publicKey)
	require.NoError(t, err)
	record := discoveryapi.CatalogRecordSnapshot{
		Version: discoveryrecords.Version,
		Node: &discoveryapi.CatalogNodeFactsSnapshot{
			Principal: principal, PublicKey: publicKey, Endpoints: append([]string(nil), endpoints...),
		},
		IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	signTerminalDiscoveryRecord(t, &record, private)
	return record, private
}

func signTerminalDiscoveryRecord(t *testing.T, record *discoveryapi.CatalogRecordSnapshot, private ed25519.PrivateKey) {
	t.Helper()
	payload, err := discoveryapi.Canonical(discoveryapi.RecordFromSnapshot(*record))
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
}

func timestampProto(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func findTransferByResource(t *testing.T, transfers []*ardentsv1.TransferSnapshot, resourceID string) *ardentsv1.TransferSnapshot {
	t.Helper()
	for _, item := range transfers {
		if item.GetResourceId() == resourceID {
			return item
		}
	}
	require.FailNow(t, "expected transfer for resource", "resource_id=%s", resourceID)
	return nil
}

const seedRecoverableOperation = `{"operations":[{"id":"op-1","kind":"node.startup.workloads","state":"running","domain":"workload","resource":"workloads","recoverable":true,"recovery_action":"restart node","started_at":"2026-03-20T10:00:00Z","updated_at":"2026-03-20T10:00:00Z"}]}`

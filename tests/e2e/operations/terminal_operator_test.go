//go:build e2e

package operations_e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/internal/cli"
	cliclient "ardents/internal/cli/client"
	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	"ardents/internal/localapi/protocol"
	transport "ardents/internal/network"
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
		Tags:        []string{"e2e", "network-operator-terminal", "node", "diagnostics"},
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

		directPending := testkit.Diagnostics(first).PendingOperations()
		require.NotEmpty(t, directPending, "direct diagnostics must retain recovery truth before terminal projection")
		pending := firstTerminal.run(t, context.Background(), "diagnostics", "pending")
		require.Contains(t, pending.stdout, "diagnostics pending")
		require.Contains(t, pending.stdout, "operation: op-1")
		require.Contains(t, pending.stdout, "  state: recovering")
		require.Contains(t, pending.stdout, "  recovery_action: restart node")
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
		Tags:        []string{"e2e", "network-operator-terminal", "network", "discovery"},
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
				ProbeEndpoints: []string{fmt.Sprintf("http://127.0.0.1:%d/ready", remotePort)},
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

	scenario.Precondition("operator starts node through terminal", func(t *testing.T) {
		start := terminal.run(t, context.Background(), "node", "start")
		require.Contains(t, start.stdout, "node start")
		require.Contains(t, start.stdout, "status: completed")
	})

	scenario.Step("terminal shows node, network and discovery readiness", func(t *testing.T) {
		status := terminal.run(t, context.Background(), "node", "status")
		require.Contains(t, status.stdout, "state: ready")
		require.Contains(t, status.stdout, "ready: true")

		network := terminal.run(t, context.Background(), "network", "status")
		require.Contains(t, network.stdout, "network status")
		require.Contains(t, network.stdout, "state: ready")
		require.Contains(t, network.stdout, "profile:")

		discovery := terminal.run(t, context.Background(), "network", "discovery")
		require.Contains(t, discovery.stdout, "network discovery")
		require.Contains(t, discovery.stdout, "state: ready")
	})

	scenario.Step("terminal events and route inspection stay truthful before and after discovery import", func(t *testing.T) {
		beforeRoutes := terminal.run(t, context.Background(), "network", "routes", "--service", "echo")
		require.Contains(t, beforeRoutes.stdout, "outcome: not_found")

		watchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		eventCh := make(chan terminalResult, 1)
		go func() {
			eventCh <- terminal.run(t, watchCtx, "--watch", "node", "events", "--limit", "1")
		}()

		records, err := remote.ListRecords()
		require.NoError(t, err)
		require.NotEmpty(t, records)
		for _, record := range records {
			record.Source = "bootstrap"
			_, err := terminal.service().ImportRecord(context.Background(), testkit.AuthorizedRequest(&ardentsv1.ImportRecordRequest{
				Record: toProtoDiscoveryRecord(record),
			}))
			require.NoError(t, err)
		}

		events := <-eventCh
		require.Contains(t, events.stdout, "[discovery/imported]")

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
		response, err := http.Get(remoteEndpoint)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		require.Equal(t, http.StatusNoContent, response.StatusCode)
	})

	scenario.Assert("terminal shows shutdown truth without false ready state", func(t *testing.T) {
		stop := terminal.run(t, context.Background(), "node", "stop")
		require.Contains(t, stop.stdout, "node stop")
		require.Contains(t, stop.stdout, "status: completed")

		status := terminal.run(t, context.Background(), "node", "status")
		require.Contains(t, status.stdout, "state: stopped")

		network := terminal.run(t, context.Background(), "network", "status")
		require.Contains(t, network.stdout, "state: stopped")
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
			ProbeEndpoints: []string{fmt.Sprintf("http://127.0.0.1:%d/ready", servicePort)},
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
	addr          string
	signerFile    string
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
	return terminalHarness{
		addr: fixture.Addr, signerFile: fixture.SignerFile,
		nodePrincipal: fixture.NodePrincipal, client: fixture.Client,
		// Fixture provisioning performs enrollment-proof and session Begin
		// calls against the same in-process Unix source before CLI commands.
		authPacer: &terminalAuthPacer{begins: 2},
	}
}

func (h terminalHarness) run(t *testing.T, ctx context.Context, args ...string) terminalResult {
	t.Helper()
	h.authPacer.wait()

	t.Setenv("ARDENTS_ADDR", h.addr)
	t.Setenv("ARDENTS_SIGNER_FILE", h.signerFile)
	t.Setenv("ARDENTS_EXPECTED_PRINCIPAL", h.nodePrincipal)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(ctx, args, &stdout, &stderr)
	require.Equalf(t, 0, code, "stderr=%s", stderr.String())
	require.Empty(t, stderr.String())

	return terminalResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
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

func terminalReadyConfig(t *testing.T, port int) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestTerminalReadyHelper"},
		"env": map[string]string{"ARDENTS_TERMINAL_READY_HELPER": "1", "ARDENTS_TERMINAL_READY_ADDRESS": fmt.Sprintf("0.0.0.0:%d", port)}})
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
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil && ip.To4() != nil && ip.IsPrivate() && !ip.IsLoopback() {
			return fmt.Sprintf("http://%s:%d/ready", ip.String(), port)
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

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cliclient "ardents/internal/cli/client"
	runtimeinfra "ardents/internal/daemon"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"ardents/tests/testkit"

	"connectrpc.com/connect"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestLegacyCredentialFlagIsExplicitAndWarningContainsNoValue(t *testing.T) {
	var stderr bytes.Buffer
	cfg, rest, _, err := parseRoot([]string{"--legacy-token", "do-not-print", "node", "status"}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.LegacyWarning || cfg.Token != "do-not-print" || len(rest) != 2 {
		t.Fatalf("unexpected parse result: warning=%v rest=%v", cfg.LegacyWarning, rest)
	}
	if strings.Contains(stderr.String(), "do-not-print") {
		t.Fatal("legacy credential leaked to warning output")
	}
}

func TestIdentityCommandsAreOfflineDeterministicAndRedacted(t *testing.T) {
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "")
	t.Setenv("ARDENTS_ADDR", "://invalid-node-address")
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")

	var rootOut, rootErr bytes.Buffer
	code := runWithIO(context.Background(), []string{"--output", "json", "identity", "principal", "create", "--signer-file", rootPath}, strings.NewReader(""), &rootOut, &rootErr)
	if code != 0 {
		t.Fatalf("principal create code = %d, stderr = %s", code, rootErr.String())
	}
	var principal map[string]any
	if err := json.Unmarshal(rootOut.Bytes(), &principal); err != nil {
		t.Fatalf("principal output is not JSON: %v: %s", err, rootOut.String())
	}
	if !strings.HasPrefix(principal["principal"].(string), "p1_") {
		t.Fatalf("principal output = %v", principal)
	}

	var deviceOut, deviceErr bytes.Buffer
	code = runWithIO(context.Background(), []string{"--output", "json", "identity", "device", "create", "--root-signer-file", rootPath, "--signer-file", devicePath, "--valid-for", "24h"}, strings.NewReader(""), &deviceOut, &deviceErr)
	if code != 0 {
		t.Fatalf("device create code = %d, stderr = %s", code, deviceErr.String())
	}
	var device map[string]any
	if err := json.Unmarshal(deviceOut.Bytes(), &device); err != nil {
		t.Fatalf("device output is not JSON: %v: %s", err, deviceOut.String())
	}
	if device["principal"] != principal["principal"] || !strings.HasPrefix(device["device_id"].(string), "d1_") || !strings.HasPrefix(device["credential_id"].(string), "kc1_") {
		t.Fatalf("device output = %v", device)
	}

	rootRaw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	deviceRaw, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatal(err)
	}
	combinedOutput := rootOut.String() + rootErr.String() + deviceOut.String() + deviceErr.String()
	for _, marker := range []string{"root_private_seed", "device_private_seed", "credential\""} {
		if strings.Contains(combinedOutput, marker) {
			t.Fatalf("private bundle field leaked to output: %q", marker)
		}
	}
	if bytes.Contains(rootOut.Bytes(), rootRaw) || bytes.Contains(deviceOut.Bytes(), deviceRaw) {
		t.Fatal("signer bundle leaked to output")
	}

	var showOut, showErr bytes.Buffer
	code = runWithIO(context.Background(), []string{"identity", "device", "show", "--signer-file", devicePath}, strings.NewReader(""), &showOut, &showErr)
	if code != 0 || !strings.Contains(showOut.String(), device["device_id"].(string)) {
		t.Fatalf("device show code = %d, stdout = %s, stderr = %s", code, showOut.String(), showErr.String())
	}
}

func TestIdentityCreateRefusesExistingSignerWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity", "root.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"identity", "principal", "create", "--signer-file", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first create code = %d, stderr = %s", code, stderr.String())
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"--output", "json", "identity", "principal", "create", "--signer-file", path}, &stdout, &stderr)
	if code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("second create code = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing signer was mutated")
	}
	if bytes.Contains(stderr.Bytes(), before) {
		t.Fatal("signer bytes leaked to error output")
	}
}

func TestIdentityFilesystemFailureDoesNotRevealSignerPathOrContent(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "private-path-marker")
	if err := os.WriteFile(parentFile, []byte("private-content-marker"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parentFile, "root.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--output", "json", "identity", "principal", "create", "--signer-file", path}, &stdout, &stderr)
	if code == 0 || stdout.Len() != 0 {
		t.Fatalf("code = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
	}
	for _, secret := range []string{path, parentFile, "private-path-marker", "private-content-marker"} {
		if strings.Contains(stderr.String(), secret) {
			t.Fatalf("filesystem detail leaked to error output: %q in %s", secret, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), "signer file is unavailable") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunReturnsFailureWhenOutputCannotBeWritten(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"node", "status"}, failingWriter{}, &stderr)
	if code == 0 {
		t.Fatal("code = 0, want output failure")
	}
}

func TestRunNodeStatusJSONSuccess(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--output", "json", "node", "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"snapshot"`)) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunDiagnosticsHealthHumanSuccess(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"diagnostics", "health"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("diagnostics health")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunWorkloadListSuccess(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"workload", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("workload list")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunDataInventoryJSONSuccess(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--output", "json", "data", "inventory"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"objects"`)) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunNodeStatusHumanIncludesOperatorTruth(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"node", "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("node status")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("state:")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("ready:")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunDiagnosticsHealthWatchPrintsInitialSnapshot(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(ctx, []string{"--watch", "--interval", "10ms", "diagnostics", "health"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("diagnostics health")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("state:")) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunNetworkStatusWatchJSONPrintsSnapshotDocument(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(ctx, []string{"--output", "json", "--watch", "--interval", "10ms", "network", "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"network"`)) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRunNodeStatusIdentityPreflightMismatchFails(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--public-key", "wrong-key", "node", "status"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("code = 0, want preflight failure")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("public key mismatch")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunNodeStatusIdentityPreflightPrincipalSuccess(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	identity := fetchRuntimeIdentity(t, srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--principal", identity.GetPrincipal(), "node", "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunNodeStatusIdentityPreflightNodeMismatchFails(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--node-name", "wrong-node", "node", "status"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("code = 0, want identity preflight failure")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("node mismatch")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunShellExecutesContextAndCommand(t *testing.T) {
	srv := newCLIServer(t)
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	identity := fetchRuntimeIdentity(t, srv.URL)
	input := strings.NewReader("context\nnode status\nexit\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runWithIO(context.Background(), []string{"--principal", identity.GetPrincipal(), "shell"}, input, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "interactive shell") {
		t.Fatalf("stdout = %s", text)
	}
	if !strings.Contains(text, "current context") {
		t.Fatalf("stdout = %s", text)
	}
	if !strings.Contains(text, "node status") {
		t.Fatalf("stdout = %s", text)
	}
}

func fetchRuntimeIdentity(t *testing.T, baseURL string) *ardentsv1.IdentitySnapshot {
	t.Helper()
	httpClient := &http.Client{Timeout: time.Second}
	service := cliclient.NewService(httpClient, baseURL)
	resp, err := service.GetNodeRuntime(context.Background(), connect.NewRequest(&ardentsv1.GetNodeRuntimeRequest{}))
	if err != nil {
		t.Fatalf("GetNodeRuntime() error = %v", err)
	}
	return resp.Msg.GetRuntime().GetIdentity()
}

func newCLIServer(t *testing.T) *httptest.Server {
	t.Helper()

	n := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "cli-test",
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	}).Runtime
	principal := testkit.NewArdentsClient(t, n)
	proxy := cliPrincipalProxy{service: principal}

	mux := http.NewServeMux()
	mux.Handle(ardentsv1connect.NewNodeServiceHandler(&proxy))
	mux.Handle(ardentsv1connect.NewDiagnosticsServiceHandler(&proxy))
	mux.Handle(ardentsv1connect.NewNetworkServiceHandler(&proxy))
	mux.Handle(ardentsv1connect.NewWorkloadServiceHandler(&proxy))
	mux.Handle(ardentsv1connect.NewContentServiceHandler(&proxy))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// cliPrincipalProxy keeps these root-command tests focused on CLI behavior.
// Product calls are authorized by testkit's Principal-only client; the outer
// HTTP server only adapts the root CLI's independently constructed transport.
type cliPrincipalProxy struct {
	ardentsv1connect.UnimplementedNodeServiceHandler
	ardentsv1connect.UnimplementedDiagnosticsServiceHandler
	ardentsv1connect.UnimplementedNetworkServiceHandler
	ardentsv1connect.UnimplementedWorkloadServiceHandler
	ardentsv1connect.UnimplementedContentServiceHandler
	service cliclient.Service
}

func (p *cliPrincipalProxy) GetNodeStatus(ctx context.Context, req *connect.Request[ardentsv1.GetNodeStatusRequest]) (*connect.Response[ardentsv1.NodeStatusResponse], error) {
	return p.service.GetNodeStatus(ctx, connect.NewRequest(req.Msg))
}

func (p *cliPrincipalProxy) GetNodeRuntime(ctx context.Context, req *connect.Request[ardentsv1.GetNodeRuntimeRequest]) (*connect.Response[ardentsv1.NodeRuntimeResponse], error) {
	return p.service.GetNodeRuntime(ctx, connect.NewRequest(req.Msg))
}

func (p *cliPrincipalProxy) GetHealthSummary(ctx context.Context, req *connect.Request[ardentsv1.GetHealthSummaryRequest]) (*connect.Response[ardentsv1.HealthSummaryResponse], error) {
	return p.service.GetHealthSummary(ctx, connect.NewRequest(req.Msg))
}

func (p *cliPrincipalProxy) GetNetworkStatus(ctx context.Context, req *connect.Request[ardentsv1.GetNetworkStatusRequest]) (*connect.Response[ardentsv1.NetworkStatusResponse], error) {
	return p.service.GetNetworkStatus(ctx, connect.NewRequest(req.Msg))
}

func (p *cliPrincipalProxy) ListWorkloads(ctx context.Context, req *connect.Request[ardentsv1.ListWorkloadsRequest]) (*connect.Response[ardentsv1.ListWorkloadsResponse], error) {
	return p.service.ListWorkloads(ctx, connect.NewRequest(req.Msg))
}

func (p *cliPrincipalProxy) GetDataInventory(ctx context.Context, req *connect.Request[ardentsv1.GetDataInventoryRequest]) (*connect.Response[ardentsv1.DataInventorySnapshot], error) {
	return p.service.GetDataInventory(ctx, connect.NewRequest(req.Msg))
}

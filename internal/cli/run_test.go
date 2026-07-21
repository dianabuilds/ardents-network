package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cliclient "ardents/internal/cli/client"
	runtimeinfra "ardents/internal/daemon"
	rpcadapter "ardents/internal/localapi"
	localauth "ardents/internal/localapi/auth"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/tests/testkit"

	"connectrpc.com/connect"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestRunReturnsFailureWhenOutputCannotBeWritten(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"node", "status"}, failingWriter{}, &stderr)
	if code == 0 {
		t.Fatal("code = 0, want output failure")
	}
}

func TestRunNodeStatusJSONSuccess(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
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
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
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

func TestRunNodeStatusUnauthorizedFailure(t *testing.T) {
	srv := newCLIServer(t, "right-token")
	t.Setenv("ARDENTS_API_TOKEN", "wrong-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"node", "status"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("code = %d, want non-zero", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("message: authentication required")) &&
		!bytes.Contains(stderr.Bytes(), []byte("Unauthenticated")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunWorkloadListSuccess(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
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
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
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
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
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

func TestRunJSONFailureWritesStructuredErrorToStderrOnly(t *testing.T) {
	srv := newCLIServer(t, "right-token")
	t.Setenv("ARDENTS_API_TOKEN", "wrong-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--output", "json", "node", "status"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("code = %d, want non-zero", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %s, want empty", stdout.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte(`"message"`)) {
		t.Fatalf("stderr = %s", stderr.String())
	}
	if bytes.Contains(stderr.Bytes(), []byte("error:")) {
		t.Fatalf("stderr mixed human/json output: %s", stderr.String())
	}
}

func TestRunDiagnosticsHealthWatchPrintsInitialSnapshot(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
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
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
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
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
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
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	identity := fetchRuntimeIdentity(t, srv.URL, "test-token")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--principal", identity.GetPrincipal(), "node", "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunNodeStatusRejectsWrongNodeBindingAtServer(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"--node-name", "wrong-node", "node", "status"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("code = 0, want node binding failure")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("binding mismatch")) {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestRunNodeStatusContextScopesNarrowCredential(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	var allowedOut bytes.Buffer
	var allowedErr bytes.Buffer
	code := Run(context.Background(), []string{"--scope", "node.status", "node", "status"}, &allowedOut, &allowedErr)
	if code != 0 {
		t.Fatalf("allowed code = %d, stderr = %s", code, allowedErr.String())
	}

	var deniedOut bytes.Buffer
	var deniedErr bytes.Buffer
	code = Run(context.Background(), []string{"--scope", "node.start", "node", "status"}, &deniedOut, &deniedErr)
	if code == 0 {
		t.Fatal("code = 0, want scoped-context denial")
	}
	if !bytes.Contains(deniedErr.Bytes(), []byte("action capability required")) {
		t.Fatalf("stderr = %s", deniedErr.String())
	}
}

func TestRunShellExecutesContextAndCommand(t *testing.T) {
	srv := newCLIServer(t, "test-token")
	t.Setenv("ARDENTS_API_TOKEN", "test-token")
	t.Setenv("ARDENTS_ADDR", srv.URL)

	identity := fetchRuntimeIdentity(t, srv.URL, "test-token")
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

func fetchRuntimeIdentity(t *testing.T, baseURL, token string) *ardentsv1.IdentitySnapshot {
	t.Helper()
	httpClient := &http.Client{Timeout: time.Second}
	service := cliclient.NewService(httpClient, baseURL)
	req := connect.NewRequest(&ardentsv1.GetNodeRuntimeRequest{})
	req.Header().Set("Authorization", "Bearer "+token)
	resp, err := service.GetNodeRuntime(context.Background(), req)
	if err != nil {
		t.Fatalf("GetNodeRuntime() error = %v", err)
	}
	return resp.Msg.GetRuntime().GetIdentity()
}

func newCLIServer(t *testing.T, token string) *httptest.Server {
	t.Helper()

	n := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "cli-test",
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	}).Runtime

	mux := http.NewServeMux()
	target := n.GetNodeRuntime()
	path, handler, err := rpcadapter.NewHandler(testkit.ConnectDependencies(n), localauth.Config{
		Token: token, SubjectID: "cli-test", Capabilities: []string{"*"},
		TargetNode: target.Node.Name, TargetPrincipal: target.Identity.Principal,
	})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

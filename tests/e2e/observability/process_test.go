//go:build e2e

package observabilitye2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

//goland:noinspection ALL
func TestArddProductionObservabilityProcessBoundary(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "diagnostics", ScenarioID: "OBE-001", Suite: "e2e",
		Tags: []string{"e2e", "observability", "security", "process"}, Speed: "fast", Environment: "linux-container",
	})
	dir := t.TempDir()
	apiAddress := reserveAddress(t)
	observabilityAddress := reserveAddress(t)
	writeToken(t, filepath.Join(dir, "api-token"), "api-secret")
	writeToken(t, filepath.Join(dir, "metrics-token"), "metrics-secret")
	configPath := writeObservabilityConfig(t, dir, apiAddress, observabilityAddress)
	binary := filepath.Join(dir, "ardd")

	scenario.Precondition("build and start the real daemon entry point", func(t *testing.T) {
		build := exec.Command("go", "build", "-o", binary, "./cmd/ardd")
		buildOutput, err := build.CombinedOutput()
		require.NoError(t, err, string(buildOutput))
	})
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "ARDENTS_CONFIG_FILE="+configPath, "ARDENTS_API_TOKEN=", "ARDENTS_API_TOKEN_FILE=")
	logPath := filepath.Join(dir, "ardd.log")
	output, err := os.Create(logPath)
	require.NoError(t, err)
	command.Stdout = output
	command.Stderr = output
	t.Cleanup(func() { _ = output.Close() })
	exited := make(chan error, 1)

	scenario.Step("start the daemon and wait for process liveness", func(t *testing.T) {
		require.NoError(t, command.Start())
		go func() { exited <- command.Wait() }()
		t.Cleanup(func() { stopProcess(command) })
		err := waitForLive("http://"+observabilityAddress+"/healthz", exited)
		require.NoError(t, err, readLog(logPath))
	})

	scenario.Step("verify degraded readiness, scrape authority, and correlation", func(t *testing.T) {
		status, body, _ := requestEndpoint(t, "http://"+observabilityAddress+"/readyz", "")
		require.Equal(t, http.StatusServiceUnavailable, status)
		require.Contains(t, body, `"health":"degraded"`)
		status, _, _ = requestEndpoint(t, "http://"+observabilityAddress+"/metrics", "")
		require.Equal(t, http.StatusUnauthorized, status)
		status, body, correlation := requestEndpoint(t, "http://"+observabilityAddress+"/metrics", "metrics-secret")
		require.Equal(t, http.StatusOK, status)
		require.Contains(t, body, "ardents_node_ready 0")
		require.Contains(t, body, `ardents_node_health{state="degraded"} 1`)
		require.NotEmpty(t, correlation)
	})

	scenario.Assert("interrupt shuts down both daemon listeners", func(t *testing.T) {
		require.NoError(t, command.Process.Signal(os.Interrupt))
		select {
		case err := <-exited:
			_ = output.Close()
			require.NoError(t, err, readLog(logPath))
		case <-time.After(10 * time.Second):
			_ = command.Process.Kill()
			t.Fatal("ardd did not shut down within 10 seconds")
		}
	})
}

func writeObservabilityConfig(t *testing.T, dir, apiAddress, observabilityAddress string) string {
	t.Helper()
	doc := runtimeconfig.Defaults()
	doc.Node.Name = "observability-e2e"
	doc.Node.Profile = "local_development"
	doc.Node.DataDir = filepath.Join(dir, "data")
	doc.API.ListenAddress = apiAddress
	doc.API.TokenFile = filepath.Join(dir, "api-token")
	doc.Observability.ListenAddress = observabilityAddress
	doc.Observability.TokenFile = filepath.Join(dir, "metrics-token")
	doc.Network.BindAddress = "127.0.0.1"
	doc.Network.ListenPort = 0
	doc.Network.ReachabilityMode = "local_only"
	doc.Network.BootstrapPeers = []string{"local://bootstrap"}
	doc.Network.StorePath = filepath.Join(dir, "waku-store.db")
	doc.Network.PrivateKeyPath = filepath.Join(dir, "waku-key.json")
	doc.Workloads.Executor = "trusted-process"
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	path := filepath.Join(dir, "ardents.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

func writeToken(t *testing.T, path, value string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(value), 0o600))
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func waitForLive(url string, exited <-chan error) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			return fmt.Errorf("ardd exited before readiness: %w", err)
		default:
		}
		response, err := httpClient().Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("observability liveness did not become available")
}

func requestEndpoint(t *testing.T, url, token string) (int, string, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := httpClient().Do(request)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	require.NoError(t, err)
	return response.StatusCode, string(body), response.Header.Get("Ardents-Correlation-ID")
}

func httpClient() *http.Client { return &http.Client{Timeout: time.Second} }

func readLog(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return "daemon log unavailable"
	}
	defer func() { _ = file.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(file, 64<<10))
	return string(raw)
}

func stopProcess(command *exec.Cmd) {
	if command.ProcessState != nil && command.ProcessState.Exited() {
		return
	}
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}

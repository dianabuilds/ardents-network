//go:build e2e

package applicationapie2e_test

import (
	"ardents/internal/config"
	"ardents/tests/testkit"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplicationUsesDedicatedDaemonInterface(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "application-interface", ScenarioID: "APP-001", Suite: "e2e",
		Tags: []string{"e2e", "application-interface", "security", "process"}, Speed: "fast", Environment: "linux-container",
	})
	dir := t.TempDir()
	operatorAddress := reserveAddress(t)
	applicationAddress := reserveAddress(t)
	observabilityAddress := reserveAddress(t)
	writeSecret(t, filepath.Join(dir, "operator-token"), "operator-secret")
	writeSecret(t, filepath.Join(dir, "application-token"), "application-secret")
	configPath := writeConfig(t, dir, operatorAddress, applicationAddress, observabilityAddress)
	binaryName := "ardentsd"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(dir, binaryName)
	probeName := "application-probe"
	if runtime.GOOS == "windows" {
		probeName += ".exe"
	}
	probe := filepath.Join(dir, probeName)

	scenario.Precondition("build and start the real daemon entry point", func(t *testing.T) {
		build := exec.Command("go", "build", "-o", binary, "../../../cmd/ardentsd")
		output, err := build.CombinedOutput()
		require.NoError(t, err, string(output))
		build = exec.Command("go", "build", "-o", probe, "../../fixtures/application-probe")
		output, err = build.CombinedOutput()
		require.NoError(t, err, string(output))
	})
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "ARDENTS_CONFIG_FILE="+configPath, "ARDENTS_API_TOKEN=", "ARDENTS_API_TOKEN_FILE=")
	logPath := filepath.Join(dir, "ardentsd.log")
	output, err := os.Create(logPath)
	require.NoError(t, err)
	command.Stdout = output
	command.Stderr = output
	t.Cleanup(func() { _ = output.Close() })
	exited := make(chan error, 1)

	scenario.Step("start the daemon and wait for the application listener", func(t *testing.T) {
		require.NoError(t, command.Start())
		go func() { exited <- command.Wait() }()
		t.Cleanup(func() { stopProcess(command) })
		require.NoError(t, waitForHTTP("http://"+observabilityAddress+"/healthz", exited), readLog(logPath))
		require.NoError(t, waitForTCP(applicationAddress, exited), readLog(logPath))
	})

	scenario.Step("put and get content with an application credential", func(t *testing.T) {
		output, err := exec.Command(probe, "endpoint", "http://"+applicationAddress, filepath.Join(dir, "application-token")).CombinedOutput()
		require.NoError(t, err, string(output))
	})

	scenario.Assert("operator credentials are rejected by the Application Interface", func(t *testing.T) {
		output, err := exec.Command(probe, "expect-unauthenticated", "http://"+applicationAddress, filepath.Join(dir, "operator-token")).CombinedOutput()
		require.NoError(t, err, string(output))
	})
}

func waitForTCP(address string, exited <-chan error) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case processErr := <-exited:
			return fmt.Errorf("ardentsd exited before Application Interface readiness: %w", processErr)
		default:
		}
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			_ = connection.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Application Interface did not become ready")
}

func writeConfig(t *testing.T, dir, operatorAddress, applicationAddress, observabilityAddress string) string {
	t.Helper()
	doc := config.Defaults()
	doc.Node.Name = "application-api-e2e"
	doc.Node.Profile = "local_development"
	doc.Node.DataDir = filepath.Join(dir, "data")
	doc.API.ListenAddress = operatorAddress
	doc.API.TokenFile = filepath.Join(dir, "operator-token")
	doc.ApplicationInterface.Enabled = true
	doc.ApplicationInterface.ListenAddress = applicationAddress
	doc.ApplicationInterface.TokenFile = filepath.Join(dir, "application-token")
	doc.ApplicationInterface.Subject = "hello-application"
	doc.ApplicationInterface.Capabilities = []string{"application.content.put", "application.content.get"}
	doc.ApplicationInterface.CredentialExpiresAt = time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	doc.Observability.ListenAddress = observabilityAddress
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

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func writeSecret(t *testing.T, path, value string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(value), 0o600))
}

func waitForHTTP(url string, exited <-chan error) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			return fmt.Errorf("ardentsd exited before readiness: %w", err)
		default:
		}
		response, err := (&http.Client{Timeout: time.Second}).Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("application listener did not become available")
}

func readLog(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "daemon log unavailable"
	}
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

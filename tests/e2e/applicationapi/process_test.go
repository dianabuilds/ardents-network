//go:build e2e

package applicationapie2e_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

type applicationIdentityFile struct {
	Credential       string `json:"credential"`
	DevicePrivateKey string `json:"device_private_key"`
}

func TestApplicationUsesDedicatedPrincipalInterface(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Application process e2e requires Unix domain sockets")
	}
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "application-interface", ScenarioID: "APP-001", Suite: "e2e",
		Tags: []string{"e2e", "application-interface", "security", "process"}, Speed: "fast", Environment: "linux-container",
	})
	dir := t.TempDir()
	authorityDir := filepath.Join(dir, "authority")
	nodeDir := filepath.Join(dir, "node")
	secretDir := filepath.Join(dir, "secrets")
	identityDir := filepath.Join(dir, "identities")
	require.NoError(t, os.MkdirAll(identityDir, 0o700))
	require.NoError(t, os.Chmod(identityDir, 0o700))
	operatorSocket := filepath.Join(secretDir, "control.sock")
	applicationSocket := filepath.Join(nodeDir+"-applications", "application.sock")
	bootstrapTicket := filepath.Join(secretDir, "operator-bootstrap-ticket")
	observabilityAddress := reserveAddress(t)
	transportAddress := reserveAddress(t)

	daemonBinary := filepath.Join(dir, "ardentsd")
	cliBinary := filepath.Join(dir, "ardentsctl")
	probeBinary := filepath.Join(dir, "application-probe")
	root := repositoryRoot(t)
	for output, source := range map[string]string{
		daemonBinary: filepath.Join(root, "cmd", "ardentsd"),
		cliBinary:    filepath.Join(root, "cmd", "ardentsctl"),
		probeBinary:  filepath.Join(root, "tests", "fixtures", "application-probe"),
	} {
		build := exec.Command("go", "build", "-o", output, source)
		raw, err := build.CombinedOutput()
		require.NoError(t, err, string(raw))
	}

	var nodePrincipal string
	scenario.Precondition("provision and start the real Principal-only daemon", func(t *testing.T) {
		raw := run(t, daemonBinary, "init",
			"--authority-dir", authorityDir,
			"--node-dir", nodeDir,
			"--secret-dir", secretDir,
			"--node-name", "application-api-e2e",
			"--transport-port", portOf(t, transportAddress),
			"--runtime-data-dir", nodeDir,
			"--runtime-secret-dir", secretDir,
		)
		nodePrincipal = extractNodePrincipal(t, raw)
		configPath := filepath.Join(secretDir, "operator.json")
		setObservabilityAddress(t, configPath, observabilityAddress)
		require.NoError(t, os.MkdirAll(filepath.Dir(applicationSocket), 0o700))

		command := exec.Command(daemonBinary)
		command.Env = append(os.Environ(), "ARDENTS_CONFIG_FILE="+configPath)
		logPath := filepath.Join(dir, "ardentsd.log")
		output, err := os.Create(logPath)
		require.NoError(t, err)
		command.Stdout = output
		command.Stderr = output
		t.Cleanup(func() { _ = output.Close() })
		exited := make(chan error, 1)
		require.NoError(t, command.Start())
		go func() { exited <- command.Wait() }()
		t.Cleanup(func() { stopProcess(command) })
		require.NoError(t, waitForHTTP("http://"+observabilityAddress+"/healthz", exited), readLog(logPath))
		require.NoError(t, waitForPath(operatorSocket, exited), readLog(logPath))
		require.NoError(t, waitForPath(applicationSocket, exited), readLog(logPath))
		require.FileExists(t, bootstrapTicket)
	})

	operatorRoot := filepath.Join(identityDir, "operator-root.json")
	operatorDevice := filepath.Join(identityDir, "operator-device.json")
	var operatorPrincipal string
	scenario.Step("enroll the Operator Principal and issue a one-use Application ticket", func(t *testing.T) {
		raw := run(t, cliBinary, "--output", "json", "identity", "principal", "create", "--signer-file", operatorRoot)
		var principalView struct {
			Principal string `json:"principal"`
		}
		require.NoError(t, json.Unmarshal(raw, &principalView))
		operatorPrincipal = principalView.Principal
		require.NotEmpty(t, operatorPrincipal)
		run(t, cliBinary, "--output", "json", "identity", "device", "create",
			"--root-signer-file", operatorRoot, "--signer-file", operatorDevice, "--valid-for", "24h")
		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "enroll",
			"--root-signer-file", operatorRoot, "--device-signer-file", operatorDevice,
			"--bootstrap-ticket-file", bootstrapTicket)
		require.NoFileExists(t, bootstrapTicket)
	})

	appIdentityPath := filepath.Join(identityDir, "application-identity.json")
	appRootPath := filepath.Join(identityDir, "application-root.json")
	appPrincipal := strings.TrimSpace(string(run(t, probeBinary, "create", appIdentityPath, appRootPath)))
	require.Regexp(t, `^p1_[a-z2-7]{52}$`, appPrincipal)
	ticketPath := filepath.Join(identityDir, "application-enrollment-ticket")
	scenario.Step("enroll the Application Principal through its dedicated listener", func(t *testing.T) {
		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "application-ticket", "issue",
			"--principal", appPrincipal,
			"--action", "application.content.put", "--action", "application.content.get",
			"--out-file", ticketPath, "--yes")
		run(t, probeBinary, "enroll", applicationSocket, nodePrincipal, ticketPath, appIdentityPath, appRootPath)
	})

	scenario.Step("put and get content with an Application Principal session", func(t *testing.T) {
		run(t, probeBinary, "use", applicationSocket, nodePrincipal, appIdentityPath)
	})

	scenario.Assert("an Operator credential cannot establish an Application session without an Application grant", func(t *testing.T) {
		operatorIdentityPath := applicationIdentityFromDeviceBundle(t, identityDir, operatorDevice)
		command := exec.Command(probeBinary, "use", applicationSocket, nodePrincipal, operatorIdentityPath)
		raw, err := command.CombinedOutput()
		require.Error(t, err, string(raw))
		require.NotContains(t, string(raw), operatorPrincipal)
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	require.NoError(t, err)
	for {
		info, statErr := os.Stat(filepath.Join(current, "go.mod"))
		if statErr == nil && info.Mode().IsRegular() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			require.FailNow(t, "repository root with go.mod was not found")
		}
		current = parent
	}
}

func applicationIdentityFromDeviceBundle(t *testing.T, dir, bundlePath string) string {
	t.Helper()
	raw, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	var bundle struct {
		DevicePrivateSeed string `json:"device_private_seed"`
		Credential        string `json:"credential"`
	}
	require.NoError(t, json.Unmarshal(raw, &bundle))
	seed, err := base64.RawURLEncoding.DecodeString(bundle.DevicePrivateSeed)
	require.NoError(t, err)
	require.Len(t, seed, ed25519.SeedSize)
	path := filepath.Join(dir, "operator-application-attempt.json")
	writeApplicationIdentity(t, path, bundle.Credential, ed25519.NewKeyFromSeed(seed))
	return path
}

func writeApplicationIdentity(t *testing.T, path, credential string, device ed25519.PrivateKey) {
	t.Helper()
	raw, err := json.Marshal(applicationIdentityFile{
		Credential: credential, DevicePrivateKey: base64.RawURLEncoding.EncodeToString(device),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

func setObservabilityAddress(t *testing.T, path, address string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(raw, &document))
	document["observability"].(map[string]any)["listen_address"] = address
	raw, err = json.Marshal(document)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

func extractNodePrincipal(t *testing.T, raw []byte) string {
	t.Helper()
	match := regexp.MustCompile(`principal=(p1_[a-z2-7]+)`).FindSubmatch(raw)
	require.Len(t, match, 2, string(raw))
	return string(match[1])
}

func run(t *testing.T, binary string, args ...string) []byte {
	t.Helper()
	command := exec.Command(binary, args...)
	raw, err := command.CombinedOutput()
	require.NoError(t, err, string(raw))
	return raw
}

func waitForPath(path string, exited <-chan error) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case processErr := <-exited:
			return fmt.Errorf("ardentsd exited before socket readiness: %w", processErr)
		default:
		}
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Unix socket did not become ready")
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

func portOf(t *testing.T, address string) string {
	t.Helper()
	_, port, err := net.SplitHostPort(address)
	require.NoError(t, err)
	return port
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
	return fmt.Errorf("observability listener did not become available")
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

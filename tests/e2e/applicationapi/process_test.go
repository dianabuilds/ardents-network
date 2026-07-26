//go:build e2e

package applicationapie2e_test

import (
	"crypto/ed25519"
	"crypto/rand"
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

	discoveryapi "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

type applicationIdentityFile struct {
	Credential       string `json:"credential"`
	DevicePrivateKey string `json:"device_private_key"`
}

type applicationContentReference struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type applicationDiscoveryTarget struct {
	ServiceID string `json:"ServiceID"`
	Endpoint  string `json:"Endpoint"`
	Scheme    string `json:"Scheme"`
}

type daemonProcess struct {
	command *exec.Cmd
	exited  chan error
	output  *os.File
}

func TestApplicationUsesDedicatedPrincipalInterface(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Application process e2e requires Unix domain sockets")
	}
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "application-interface", ScenarioID: "APP-001", Suite: "e2e",
		Tags: []string{"e2e", "application-interface", "application-discovery", "security", "process", "retry", "restart", "revocation"}, Speed: "fast", Environment: "linux-container",
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
	configPath := filepath.Join(secretDir, "operator.json")
	logPath := filepath.Join(dir, "ardentsd.log")
	discoveryRecord, discoveryPublisher, discoveryPublic := signedApplicationDiscoveryRecord(t)

	daemonBinary := filepath.Join(dir, "ardentsd")
	faultDaemonBinary := filepath.Join(dir, "ardentsd-enrollment-fault")
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
	buildFaultDaemon := exec.Command("go", "build",
		"-ldflags", "-X ardents/internal/identity/access.applicationEnrollmentTransactionFaultMode=once",
		"-o", faultDaemonBinary, filepath.Join(root, "cmd", "ardentsd"),
	)
	raw, err := buildFaultDaemon.CombinedOutput()
	require.NoError(t, err, string(raw))

	var nodePrincipal string
	var daemon *daemonProcess
	t.Cleanup(func() {
		if daemon != nil {
			daemon.stop()
		}
	})
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
		setObservabilityAddress(t, configPath, observabilityAddress)
		setDiscoveryTrust(t, configPath, discoveryPublisher, discoveryPublic)
		seedImportedDiscoveryRecord(t, nodeDir, discoveryRecord, discoveryPublisher, discoveryPublic)
		require.NoError(t, os.MkdirAll(filepath.Dir(applicationSocket), 0o700))
		daemon = startDaemon(t, daemonBinary, configPath, logPath)
		require.NoError(t, waitForHTTP("http://"+observabilityAddress+"/healthz", daemon.exited), readLog(logPath))
		require.NoError(t, waitForPath(operatorSocket, daemon.exited), readLog(logPath))
		require.NoError(t, waitForPath(applicationSocket, daemon.exited), readLog(logPath))
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
	appDeviceID := strings.TrimSpace(string(run(t, probeBinary, "device", appIdentityPath)))
	require.Regexp(t, `^d1_[a-z2-7]{52}$`, appDeviceID)
	staleTicketPath := filepath.Join(identityDir, "stale-application-enrollment-ticket")
	ticketPath := filepath.Join(identityDir, "application-enrollment-ticket")
	var applicationGrantID string
	scenario.Step("replace a stale ticket and safely retry the current ticket after a pre-commit Node failure", func(t *testing.T) {
		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "application-ticket", "issue",
			"--principal", appPrincipal,
			"--action", "application.content.put", "--action", "application.content.get",
			"--out-file", staleTicketPath, "--yes")
		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "application-ticket", "issue",
			"--principal", appPrincipal,
			"--action", "application.content.put", "--action", "application.content.get",
			"--out-file", ticketPath, "--yes")
		staleFailure := runError(t, probeBinary, "enroll", applicationSocket, nodePrincipal, staleTicketPath, appIdentityPath, appRootPath)
		require.NotContains(t, string(staleFailure), appPrincipal)
		require.FileExists(t, staleTicketPath)

		daemon.stop()
		daemon = nil
		daemon = startDaemon(t, faultDaemonBinary, configPath, logPath)
		require.NoError(t, waitForHTTP("http://"+observabilityAddress+"/healthz", daemon.exited), readLog(logPath))
		require.NoError(t, waitForPath(operatorSocket, daemon.exited), readLog(logPath))
		require.NoError(t, waitForPath(applicationSocket, daemon.exited), readLog(logPath))
		runError(t, probeBinary, "enroll", applicationSocket, nodePrincipal, ticketPath, appIdentityPath, appRootPath)
		require.FileExists(t, ticketPath)

		var enrollment struct {
			GrantID string `json:"grant_id"`
		}
		require.NoError(t, json.Unmarshal(run(t, probeBinary, "enroll", applicationSocket, nodePrincipal, ticketPath, appIdentityPath, appRootPath), &enrollment))
		applicationGrantID = enrollment.GrantID
		require.Regexp(t, `^ag1_[a-z2-7]{52}$`, applicationGrantID)
		require.NoFileExists(t, ticketPath)
	})

	scenario.Step("resolve a trusted imported service through the protected Application socket", func(t *testing.T) {
		discoveryIdentityPath := filepath.Join(identityDir, "discovery-application-identity.json")
		discoveryRootPath := filepath.Join(identityDir, "discovery-application-root.json")
		discoveryPrincipal := strings.TrimSpace(string(run(t, probeBinary,
			"create", discoveryIdentityPath, discoveryRootPath)))
		discoveryTicketPath := filepath.Join(identityDir, "discovery-application-enrollment-ticket")
		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "application-ticket", "issue",
			"--principal", discoveryPrincipal, "--action", "application.discovery.resolve",
			"--out-file", discoveryTicketPath, "--yes")
		run(t, probeBinary, "enroll", applicationSocket, nodePrincipal, discoveryTicketPath,
			discoveryIdentityPath, discoveryRootPath)
		var targets []applicationDiscoveryTarget
		require.NoError(t, json.Unmarshal(run(t, probeBinary, "discover",
			applicationSocket, nodePrincipal, discoveryIdentityPath, "echo", "https"), &targets))
		require.Equal(t, []applicationDiscoveryTarget{{
			ServiceID: "svc.remote.echo",
			Endpoint:  "https://10.20.30.40:8443",
			Scheme:    "https",
		}}, targets)
		require.NotContains(t, string(run(t, probeBinary, "discover",
			applicationSocket, nodePrincipal, discoveryIdentityPath, "echo", "https")), operatorPrincipal)
	})

	scenario.Step("put and get content with an Application Principal session", func(t *testing.T) {
		run(t, probeBinary, "use", applicationSocket, nodePrincipal, appIdentityPath)
	})

	var durableReference applicationContentReference
	scenario.Step("reauthenticate after Application and Node restart and get existing content", func(t *testing.T) {
		require.NoError(t, json.Unmarshal(run(t, probeBinary, "put", applicationSocket, nodePrincipal, appIdentityPath, "durable application payload"), &durableReference))
		require.NotEmpty(t, durableReference.Kind)
		require.NotEmpty(t, durableReference.ID)

		daemon.stop()
		daemon = nil
		daemon = startDaemon(t, daemonBinary, configPath, logPath)
		require.NoError(t, waitForHTTP("http://"+observabilityAddress+"/healthz", daemon.exited), readLog(logPath))
		require.NoError(t, waitForPath(operatorSocket, daemon.exited), readLog(logPath))
		require.NoError(t, waitForPath(applicationSocket, daemon.exited), readLog(logPath))
		run(t, probeBinary, "get", applicationSocket, nodePrincipal, appIdentityPath, durableReference.Kind, durableReference.ID, "durable application payload")
	})

	scenario.Step("distinguish live-session grant revocation from device revocation", func(t *testing.T) {
		grantReady := filepath.Join(dir, "grant-ready")
		grantContinue := filepath.Join(dir, "grant-continue")
		deviceReady := filepath.Join(dir, "device-ready")
		deviceContinue := filepath.Join(dir, "device-continue")
		command := exec.Command(probeBinary, "observe-revocation",
			applicationSocket, nodePrincipal, appIdentityPath,
			durableReference.Kind, durableReference.ID,
			grantReady, grantContinue, deviceReady, deviceContinue)
		raw := &strings.Builder{}
		command.Stdout = raw
		command.Stderr = raw
		require.NoError(t, command.Start())
		probeExited := make(chan error, 1)
		go func() { probeExited <- command.Wait() }()
		require.NoError(t, waitForRegularFile(grantReady, probeExited), raw.String())

		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "grant", "revoke",
			"--subject", appPrincipal, "--grant-id", applicationGrantID, "--yes")
		signal(t, grantContinue)
		require.NoError(t, waitForRegularFile(deviceReady, probeExited), raw.String())

		run(t, cliBinary,
			"--addr", "unix://"+operatorSocket, "--principal", nodePrincipal, "--signer-file", operatorDevice,
			"--output", "json", "identity", "device", "revoke",
			"--principal", appPrincipal, "--device-id", appDeviceID, "--yes")
		signal(t, deviceContinue)
		require.NoError(t, <-probeExited, raw.String())
		require.Contains(t, raw.String(), `"grant":"forbidden"`)
		require.Contains(t, raw.String(), `"device":"unauthenticated"`)
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

func signedApplicationDiscoveryRecord(t *testing.T) (discoveryapi.Record, string, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	encodedPublic := base64.StdEncoding.EncodeToString(public)
	now := time.Now().UTC().Truncate(time.Second).Add(-time.Second)
	record := discoveryapi.Record{
		Version: discoveryrecord.Version,
		Service: &discoveryrecord.ServiceFacts{
			ID:            "svc.remote.echo",
			Type:          "echo",
			NodePrincipal: principal,
			Workload:      "work.remote.echo",
			Mode:          "NetworkPublished",
			PublicKey:     encodedPublic,
			Endpoints:     []string{"https://10.20.30.40:8443"},
		},
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
	}
	payload, err := discoveryapi.Canonical(record)
	require.NoError(t, err)
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	clear(private)
	record.Signature = signature
	return record, principal.String(), encodedPublic
}

func seedImportedDiscoveryRecord(
	t *testing.T,
	dir string,
	record discoveryapi.Record,
	principal string,
	encodedPublic string,
) {
	t.Helper()
	public, err := base64.StdEncoding.DecodeString(encodedPublic)
	require.NoError(t, err)
	registry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: principal,
		PublicKey: ed25519.PublicKey(public),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}})
	require.NoError(t, err)
	trust := discoveryapi.NewTrustEvaluator(registry)
	store := discoveryapi.NewInDirWithTrust(dir, trust)
	result, err := store.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)
	require.True(t, result.Applied)
}

func setDiscoveryTrust(t *testing.T, path, principal, publicKey string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(raw, &document))
	trust, ok := document["trust"].(map[string]any)
	require.True(t, ok)
	principals, ok := trust["principals"].([]any)
	require.True(t, ok)
	trust["principals"] = append(principals, map[string]any{
		"principal":  principal,
		"public_key": publicKey,
		"purposes":   []string{"discovery.publish"},
	})
	raw, err = json.Marshal(document)
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

func runError(t *testing.T, binary string, args ...string) []byte {
	t.Helper()
	command := exec.Command(binary, args...)
	raw, err := command.CombinedOutput()
	require.Error(t, err, string(raw))
	return raw
}

func startDaemon(t *testing.T, binary, configPath, logPath string) *daemonProcess {
	t.Helper()
	command := exec.Command(binary)
	command.Env = append(os.Environ(), "ARDENTS_CONFIG_FILE="+configPath)
	output, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	require.NoError(t, err)
	command.Stdout = output
	command.Stderr = output
	exited := make(chan error, 1)
	require.NoError(t, command.Start())
	go func() { exited <- command.Wait() }()
	return &daemonProcess{command: command, exited: exited, output: output}
}

func (p *daemonProcess) stop() {
	if p == nil || p.command == nil {
		return
	}
	if p.command.ProcessState == nil || !p.command.ProcessState.Exited() {
		_ = p.command.Process.Kill()
		<-p.exited
	}
	_ = p.output.Close()
	p.command = nil
}

func signal(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("continue"), 0o600))
}

func waitForRegularFile(path string, exited <-chan error) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case processErr := <-exited:
			return fmt.Errorf("Application probe exited before lifecycle signal: %w", processErr)
		default:
		}
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Application probe lifecycle signal was not created")
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

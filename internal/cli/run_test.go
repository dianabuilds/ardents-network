package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	identityprincipal "ardents/internal/identity/principal"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestLegacyCredentialFlagIsRejectedWithoutValueLeak(t *testing.T) {
	var stderr bytes.Buffer
	_, _, _, err := parseRoot([]string{"--legacy-token", "do-not-print", "node", "status"}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("parseRoot() error = %v, want unknown legacy flag rejection", err)
	}
	if strings.Contains(stderr.String(), "do-not-print") || strings.Contains(err.Error(), "do-not-print") {
		t.Fatal("rejected legacy credential value leaked to output")
	}
}

func TestIdentityCommandsAreOfflineDeterministicAndRedacted(t *testing.T) {
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
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"version"}, failingWriter{}, &stderr)
	if code == 0 {
		t.Fatal("code = 0, want output failure")
	}
}

func TestHelpTreeIsReachableWithoutOperatorContext(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{name: "root", args: []string{"--help"}, want: []string{"node", "network", "identity", "shell", "tui"}},
		{name: "node group", args: []string{"node", "help"}, want: []string{"node start", "node events [--limit N]", "requires global --watch"}},
		{name: "network group", args: []string{"network", "help"}, want: []string{"network resolve record --subject ID --kind KIND", "network records import --file FILE"}},
		{name: "workload group", args: []string{"workload", "help"}, want: []string{"workload register --file FILE", "workload publication ID"}},
		{name: "data group", args: []string{"data", "help"}, want: []string{"data objects publish --file FILE", "data transfers get ID"}},
		{name: "diagnostics group", args: []string{"diagnostics", "help"}, want: []string{"diagnostics explain --scope S [--resource-id ID]"}},
		{name: "config group", args: []string{"config", "help"}, want: []string{"config show", "config reload"}},
		{name: "identity group", args: []string{"identity", "help"}, want: []string{"identity principal create [--signer-file PATH]", "identity grant issue"}},
		{name: "shell leaf", args: []string{"shell", "help"}, want: []string{"Usage:", "shell"}},
		{name: "tui leaf", args: []string{"tui", "help"}, want: []string{"Usage:", "tui"}},
		{name: "network resolve nested", args: []string{"network", "resolve", "help"}, want: []string{"network resolve record --subject ID --kind KIND", "network resolve service --service ID"}},
		{name: "network records nested", args: []string{"network", "records", "help"}, want: []string{"network records list", "network records import --file FILE"}},
		{name: "data objects nested", args: []string{"data", "objects", "help"}, want: []string{"data objects list", "data objects get ID", "data objects publish --file FILE"}},
		{name: "data blobs nested", args: []string{"data", "blobs", "help"}, want: []string{"data blobs retain --id ID --expires-at TIME"}},
		{name: "data manifests nested", args: []string{"data", "manifests", "help"}, want: []string{"data manifests publish --file FILE"}},
		{name: "data transfers nested", args: []string{"data", "transfers", "help"}, want: []string{"data transfers list", "data transfers get ID"}},
		{name: "identity principal nested", args: []string{"identity", "principal", "help"}, want: []string{"identity principal import --from-file PATH"}},
		{name: "identity device nested", args: []string{"identity", "device", "help"}, want: []string{"identity device revoke --principal ID --device-id ID"}},
		{name: "identity grant nested", args: []string{"identity", "grant", "help"}, want: []string{"identity grant list --subject ID", "identity grant revoke --subject ID --grant-id ID"}},
		{name: "identity delegation nested", args: []string{"identity", "delegation", "help"}, want: []string{"identity delegation import-revocation --revocation-file PATH"}},
		{name: "identity application ticket nested", args: []string{"identity", "application-ticket", "help"}, want: []string{"identity application-ticket issue --principal ID"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runWithIO(context.Background(), testCase.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runWithIO(%v) code = %d, stderr = %q", testCase.args, code, stderr.String())
			}
			for _, want := range testCase.want {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("runWithIO(%v) output missing %q:\n%s", testCase.args, want, stdout.String())
				}
			}
		})
	}
}

func TestUnknownHelpEntryFailsClosedBeforeContextResolution(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWithIO(context.Background(), []string{"network", "unknown", "help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `unknown help path "network unknown"`) {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestHelpTokenRemainsAvailableAsLeafPositionalArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code, handled := renderHelpIfRequested([]string{"workload", "get", "help"}, &stdout, &stderr); handled || code != 0 {
		t.Fatalf("leaf positional argument was intercepted: handled=%t code=%d stderr=%q", handled, code, stderr.String())
	}
}

func TestIncompleteNestedCommandFailsClosedWithoutContextResolution(t *testing.T) {
	for _, args := range [][]string{{"network", "resolve"}, {"data", "objects"}, {"identity", "grant"}} {
		var stdout, stderr bytes.Buffer
		code := runWithIO(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), "Commands:") {
			t.Fatalf("runWithIO(%v) code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
}

func TestHumanOnlyCommandsRejectJSONBeforeContextResolution(t *testing.T) {
	for _, commandName := range []string{"shell", "tui"} {
		t.Run(commandName, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runWithIO(context.Background(), []string{"--output", "json", commandName}, strings.NewReader(""), &stdout, &stderr)
			if code != 2 || !strings.Contains(stderr.String(), commandName+" does not support --output=json") {
				t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestProtectedCommandRejectsLegacyBearerEnvironment(t *testing.T) {
	t.Setenv("ARDENTS_ADDR", "unix:///run/ardents/operator.sock")
	t.Setenv("ARDENTS_SIGNER_FILE", "")
	t.Setenv("ARDENTS_EXPECTED_PRINCIPAL", "")
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "do-not-print")
	t.Setenv("ARDENTS_LEGACY_TOKEN_FILE", filepath.Join(t.TempDir(), "legacy-token"))

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--context-file", filepath.Join(t.TempDir(), "missing.json"), "node", "status"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "obsolete credential environment variable") {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "do-not-print") {
		t.Fatal("legacy bearer leaked to error output")
	}
}

func TestProtectedCommandRequiresTargetNodePrincipal(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--context-file", filepath.Join(t.TempDir(), "missing.json"),
		"--addr", "unix:///run/ardents/operator.sock",
		"--signer-file", filepath.Join(t.TempDir(), "device.json"),
		"node", "status",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "target Node Principal") {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
}

func TestProtectedCommandRejectsPrincipalSignerOverPlaintext(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"--context-file", filepath.Join(t.TempDir(), "missing.json"),
		"--addr", "http://127.0.0.1:18080",
		"--principal", cliTestPrincipal(t),
		"--signer-file", filepath.Join(t.TempDir(), "device.json"),
		"node", "status",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "protected Unix socket") {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
}

func cliTestPrincipal(t *testing.T) string {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	return principal.String()
}

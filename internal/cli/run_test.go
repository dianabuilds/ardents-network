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

func TestProtectedCommandRejectsLegacyBearerEnvironment(t *testing.T) {
	t.Setenv("ARDENTS_ADDR", "http://127.0.0.1:18080")
	t.Setenv("ARDENTS_SIGNER_FILE", "")
	t.Setenv("ARDENTS_EXPECTED_PRINCIPAL", "")
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "do-not-print")
	t.Setenv("ARDENTS_LEGACY_TOKEN_FILE", filepath.Join(t.TempDir(), "legacy-token"))

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--context-file", filepath.Join(t.TempDir(), "missing.json"), "node", "status"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "operator address must use a protected Unix socket") {
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

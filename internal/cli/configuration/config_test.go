package configuration

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestConfigResolvePrincipalFromEnv(t *testing.T) {
	node := configTestPrincipal(t)
	signerFile := filepath.Join(t.TempDir(), "device.json")
	t.Setenv("ARDENTS_ADDR", "unix:///run/ardents/operator.sock")
	t.Setenv("ARDENTS_SIGNER_FILE", signerFile)
	t.Setenv("ARDENTS_EXPECTED_PRINCIPAL", node)
	t.Setenv("ARDENTS_SCOPE_HINTS", "node.status,diagnostics.snapshot")
	t.Setenv("ARDENTS_OUTPUT", "json")
	t.Setenv("ARDENTS_TIMEOUT", "3s")

	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "unix:///run/ardents/operator.sock" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.SignerFile != signerFile || cfg.ExpectedPrincipal != node {
		t.Fatalf("Principal configuration = signer %q principal %q", cfg.SignerFile, cfg.ExpectedPrincipal)
	}
	if cfg.Output != "json" || cfg.Timeout != 3*time.Second {
		t.Fatalf("presentation configuration = output %q timeout %v", cfg.Output, cfg.Timeout)
	}
	if len(cfg.ScopeHints) != 2 {
		t.Fatalf("ScopeHints = %#v", cfg.ScopeHints)
	}
}

func TestDefaultConfigUsesProtectedOperatorSocket(t *testing.T) {
	cfg := DefaultConfig()
	require.Equal(t, "unix:///run/ardents/control.sock", cfg.Addr)
}

func TestConfigResolvePrincipalFromContextFile(t *testing.T) {
	dir := t.TempDir()
	contextFile := filepath.Join(dir, "contexts.json")
	node := configTestPrincipal(t)
	signerFile := filepath.Join(dir, "device.json")
	raw := `{
  "default": "local",
  "contexts": {
    "local": {
      "addr": "unix:///run/ardents/operator.sock",
      "signer_file": "` + filepath.ToSlash(signerFile) + `",
      "expected_principal": "` + node + `",
      "expected_public_key": "pub-1",
      "scope_hints": ["node.status"],
      "timeout": "4s"
    }
  }
}`
	if err := os.WriteFile(contextFile, []byte(raw), 0o600); err != nil {
		t.Fatalf("WriteFile(contexts): %v", err)
	}

	cfg := DefaultConfig()
	cfg.ContextFile = contextFile
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "unix:///run/ardents/operator.sock" || cfg.SignerFile != filepath.ToSlash(signerFile) {
		t.Fatalf("transport configuration = addr %q signer %q", cfg.Addr, cfg.SignerFile)
	}
	if cfg.ExpectedPrincipal != node || cfg.ExpectedPublicKey != "pub-1" {
		t.Fatalf("identity binding = principal %q public key %q", cfg.ExpectedPrincipal, cfg.ExpectedPublicKey)
	}
	if cfg.Timeout != 4*time.Second {
		t.Fatalf("Timeout = %v", cfg.Timeout)
	}
}

func TestResolveTopologyContextUsesExactStoredBindingsWithoutEnvironmentOverrides(t *testing.T) {
	principal := configTestPrincipal(t)
	contextFile := filepath.Join(t.TempDir(), "contexts.json")
	signerFile := filepath.Join(t.TempDir(), "device.json")
	raw := fmt.Sprintf(`{
		"contexts": {
			"host-a": {
				"addr": "unix:///run/ardents/operator.sock",
				"ssh": "operator@host-a",
				"ssh_known_hosts": "pins/host-a",
				"ssh_operator_socket": "/run/ardents/operator.sock",
				"signer_file": %q,
				"signer_alias": "operator-a",
				"host_key_pin_ref": "pin-a",
				"expected_node": "node-a",
				"expected_principal": %q
			}
		}
	}`, filepath.ToSlash(signerFile), principal)
	if err := os.WriteFile(contextFile, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARDENTS_SSH", "attacker@override")
	t.Setenv("ARDENTS_SIGNER_FILE", "override.json")

	cfg, err := (Config{ContextFile: contextFile, Output: "json", Timeout: time.Second}).ResolveTopologyContext("host-a")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SSH != "operator@host-a" || cfg.SignerFile != filepath.ToSlash(signerFile) ||
		cfg.SignerAlias != "operator-a" || cfg.HostKeyPinRef != "pin-a" {
		t.Fatalf("resolved topology context = %+v", cfg)
	}
}

func TestResolveTopologyContextRequiresPinSignerAndIdentityBindings(t *testing.T) {
	contextFile := filepath.Join(t.TempDir(), "contexts.json")
	if err := os.WriteFile(contextFile, []byte(`{"contexts":{"host-a":{"ssh":"operator@host-a"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Config{ContextFile: contextFile}).ResolveTopologyContext("host-a")
	if err == nil {
		t.Fatal("ResolveTopologyContext() error = nil")
	}
}

func TestContextFileMustBeRegularAndBounded(t *testing.T) {
	_, err := (Config{ContextFile: t.TempDir()}).ResolveTopologyContext("host-a")
	require.ErrorContains(t, err, "regular file")

	path := filepath.Join(t.TempDir(), "contexts.json")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{' '}, maxContextsBytes+1), 0o600))
	_, err = (Config{ContextFile: path}).ResolveTopologyContext("host-a")
	require.ErrorContains(t, err, "size limit")
}

func TestConfigResolveRequiresPrincipalAuthentication(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	err := cfg.Resolve()
	if err == nil || !strings.Contains(err.Error(), "target Node Principal") {
		t.Fatalf("Resolve() error = %v, want target Principal requirement", err)
	}
}

func TestConfigResolveAcceptsProtectedUnixSocket(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "unix:///run/ardents/operator.sock"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "unix:///run/ardents/operator.sock" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
}

func TestConfigResolveAcceptsSSHStreamLocalForwarding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.SSH = "ops@node.example"
	cfg.SSHOperatorSocket = "/var/lib/ardents/operator.sock"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.SSHPort != 22 {
		t.Fatalf("SSHPort = %d", cfg.SSHPort)
	}
}

func TestConfigResolveRejectsLegacySSHLoopbackForwarding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "http://api.example:8080"
	cfg.SSH = "ops@node.example"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want non-stream-local SSH rejection")
	}
}

func TestConfigResolveRejectsPrincipalSignerOverHTTP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "http://127.0.0.1:8080"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err == nil || !strings.Contains(err.Error(), "operator address must use a protected Unix socket") {
		t.Fatalf("Resolve() error = %v, want HTTP rejection", err)
	}
}

func TestConfigResolveRejectsSSHWithoutStreamLocalSocket(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.SSH = "ops@node.example"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")

	err := cfg.Resolve()
	if err == nil || !strings.Contains(err.Error(), "SSH transport requires an absolute remote Operator Unix socket") {
		t.Fatalf("Resolve() error = %v, want stream-local SSH rejection", err)
	}
}

func TestConfigResolvePrincipalSSHRequiresAbsoluteOperatorSocket(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.SSH = "ops@node.example"
	cfg.SSHOperatorSocket = "relative.sock"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want relative remote socket rejection")
	}
}

func TestObsoleteBearerEnvironmentIsRejectedWithoutSecretLeak(t *testing.T) {
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "stale-secret")
	t.Setenv("ARDENTS_LEGACY_TOKEN_FILE", filepath.Join(t.TempDir(), "stale-token"))
	t.Setenv("ARDENTS_API_TOKEN", "older-secret")
	t.Setenv("ARDENTS_TOKEN_FILE", filepath.Join(t.TempDir(), "older-token"))

	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	err := cfg.Resolve()
	if err == nil || !strings.Contains(err.Error(), "obsolete credential environment variable") {
		t.Fatalf("Resolve() error = %v, want obsolete credential rejection", err)
	}
	for _, secret := range []string{"stale-secret", "older-secret"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("legacy secret leaked in error: %q", secret)
		}
	}
}

func TestLegacyBearerContextFieldIsRejectedAsUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contexts.json")
	secret := "stale-secret-name"
	raw := `{"default":"local","contexts":{"local":{"addr":"unix:///run/ardents/operator.sock","legacy_token_env":"` + secret + `"}}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.ContextFile = path
	err := cfg.Resolve()
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Resolve() error = %v, want unknown legacy field rejection", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("legacy field value leaked in parse error")
	}
}

func configTestPrincipal(t *testing.T) string {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x77}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	return principal.String()
}

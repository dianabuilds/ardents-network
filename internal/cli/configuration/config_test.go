package configuration

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

func TestConfigResolveFromEnv(t *testing.T) {
	t.Setenv("ARDENTS_ADDR", "127.0.0.1:18080")
	t.Setenv("ARDENTS_LEGACY_API_TOKEN", "env-token")
	t.Setenv("ARDENTS_SCOPE_HINTS", "node.status,diagnostics.snapshot")
	t.Setenv("ARDENTS_OUTPUT", "json")
	t.Setenv("ARDENTS_TIMEOUT", "3s")

	cfg := DefaultConfig()
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "http://127.0.0.1:18080" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.Token != "env-token" {
		t.Fatalf("Token = %q", cfg.Token)
	}
	if !cfg.LegacyWarning {
		t.Fatal("legacy environment selection did not request a warning")
	}
	if cfg.Output != "json" {
		t.Fatalf("Output = %q", cfg.Output)
	}
	if cfg.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v", cfg.Timeout)
	}
	if len(cfg.ScopeHints) != 2 {
		t.Fatalf("ScopeHints = %#v", cfg.ScopeHints)
	}
}

func TestConfigResolveFromContextFile(t *testing.T) {
	dir := t.TempDir()
	contextFile := filepath.Join(dir, "contexts.json")
	if err := os.WriteFile(contextFile, []byte(`{
  "default": "local",
  "contexts": {
    "local": {
      "addr": "127.0.0.1:19090",
      "legacy_token_file": "`+filepath.ToSlash(filepath.Join(dir, "token.txt"))+`",
      "expected_principal": "principal-1",
      "expected_public_key": "pub-1",
      "scope_hints": ["node.status"],
      "timeout": "4s"
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("WriteFile(contexts): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "token.txt"), []byte("file-token\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(token): %v", err)
	}

	cfg := DefaultConfig()
	cfg.ContextFile = contextFile
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "http://127.0.0.1:19090" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.Token != "file-token" {
		t.Fatalf("Token = %q", cfg.Token)
	}
	if cfg.ExpectedPrincipal != "principal-1" {
		t.Fatalf("ExpectedPrincipal = %q", cfg.ExpectedPrincipal)
	}
	if cfg.ExpectedPublicKey != "pub-1" {
		t.Fatalf("ExpectedPublicKey = %q", cfg.ExpectedPublicKey)
	}
	if cfg.Timeout != 4*time.Second {
		t.Fatalf("Timeout = %v", cfg.Timeout)
	}
}

func TestConfigResolveRequiresToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	err := cfg.Resolve()
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing token")
	}
}

func TestConfigResolveAcceptsUnixSocket(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "unix:///run/ardents/control.sock"
	cfg.Token = "token"
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "unix:///run/ardents/control.sock" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
}

func TestConfigResolveAcceptsSSHToLoopbackAPI(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "127.0.0.1:8080"
	cfg.SSH = "ops@node.example"
	cfg.Token = "token"
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.Addr != "http://127.0.0.1:8080" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
}

func TestConfigResolveRejectsSSHToRemoteAPIAddress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "http://api.example:8080"
	cfg.SSH = "ops@node.example"
	cfg.Token = "token"
	if err := cfg.Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want remote API rejection")
	}
}

func TestConfigResolveSelectsPrincipalOnlyForProtectedLocalTransport(t *testing.T) {
	node := configTestPrincipal(t)
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "unix:///run/ardents/operator.sock"
	cfg.ExpectedPrincipal = node
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.AuthMode() != AuthModePrincipal {
		t.Fatalf("AuthMode() = %q", cfg.AuthMode())
	}

	httpConfig := DefaultConfig()
	httpConfig.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	httpConfig.Addr = "http://127.0.0.1:8080"
	httpConfig.ExpectedPrincipal = node
	httpConfig.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := httpConfig.Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want Principal-over-HTTP rejection")
	}
}

func TestConfigResolvePrincipalSSHRequiresAbsoluteOperatorSocket(t *testing.T) {
	node := configTestPrincipal(t)
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.SSH = "ops@node.example"
	cfg.SSHOperatorSocket = "/var/lib/ardents/secrets/control.sock"
	cfg.ExpectedPrincipal = node
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.AuthMode() != AuthModePrincipal {
		t.Fatalf("AuthMode() = %q", cfg.AuthMode())
	}

	cfg.SSHOperatorSocket = "relative.sock"
	if err := cfg.Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want relative remote socket rejection")
	}
}

func TestConfigResolveRejectsMixedPrincipalAndLegacyCredentials(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "unix:///run/ardents/operator.sock"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	cfg.SignerFile = filepath.Join(t.TempDir(), "device.json")
	cfg.Token = "legacy"
	if err := cfg.Resolve(); err == nil {
		t.Fatal("Resolve() error = nil, want mixed authentication rejection")
	}
}

func TestOldAmbientBearerNamesCannotDowngradePrincipalDefault(t *testing.T) {
	t.Setenv("ARDENTS_API_TOKEN", "stale-ambient-bearer")
	t.Setenv("ARDENTS_TOKEN_FILE", filepath.Join(t.TempDir(), "stale-token"))
	cfg := DefaultConfig()
	cfg.ContextFile = filepath.Join(t.TempDir(), "missing.json")
	cfg.Addr = "unix:///run/ardents/operator.sock"
	cfg.ExpectedPrincipal = configTestPrincipal(t)
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.AuthMode() != AuthModePrincipal || cfg.Token != "" || cfg.SignerFile == "" {
		t.Fatalf("old ambient bearer selected mode=%q token_set=%v signer_set=%v", cfg.AuthMode(), cfg.Token != "", cfg.SignerFile != "")
	}
}

func TestOldContextBearerFieldsCannotDowngradePrincipalDefault(t *testing.T) {
	dir := t.TempDir()
	contextFile := filepath.Join(dir, "contexts.json")
	raw := `{"default":"alpha","contexts":{"alpha":{"addr":"unix:///run/ardents/operator.sock","expected_principal":"` + configTestPrincipal(t) + `","token_env":"OLD_TOKEN","token_file":"old-token"}}}`
	if err := os.WriteFile(contextFile, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OLD_TOKEN", "stale-context-bearer")
	cfg := DefaultConfig()
	cfg.ContextFile = contextFile
	if err := cfg.Resolve(); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if cfg.AuthMode() != AuthModePrincipal || cfg.Token != "" {
		t.Fatalf("old context bearer selected mode=%q token_set=%v", cfg.AuthMode(), cfg.Token != "")
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

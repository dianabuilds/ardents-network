package daemon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	runtimeconfig "ardents/internal/config"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TestLoadRuntimeConfigUsesVersionedOperatorDocument(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ardents.json")
	raw := `{
      "api_version":"ardents.config/v1",
      "node":{"name":"node-a","data_dir":"` + filepath.ToSlash(filepath.Join(dir, "data")) + `"},
      "api":{"socket_path":"` + filepath.ToSlash(filepath.Join(dir, "operator.sock")) + `"},
      "network":{"bootstrap_peers":["/ip4/10.0.0.2/tcp/60000/p2p/peer"]},
      "data":{"max_replica_bytes":4096,"desired_replicas":3,"minimum_replicas":2},
      "policy":{"disable_untrusted_route_use":true}
    }`
	require.NoError(t, os.WriteFile(configPath, []byte(raw), 0o600))
	t.Setenv(operatorConfigFileEnv, configPath)

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, "node-a", cfg.Node.Name)
	require.Equal(t, filepath.Join(dir, "data"), cfg.Node.Data.Dir)
	require.Equal(t, filepath.Join(dir, "data", "waku-store.db"), cfg.Node.Transport.StorePath)
	require.Equal(t, filepath.ToSlash(filepath.Join(dir, "operator.sock")), cfg.SocketPath)
	require.Equal(t, int64(4096), cfg.Node.Data.MaxReplicaRetentionBytes)
	require.True(t, cfg.Node.Policy.DisableUntrustedRouteUse)
	require.Equal(t, []string{"/ip4/10.0.0.2/tcp/60000/p2p/peer"}, cfg.Node.Boot.Sources)
}

func TestRuntimeConfigMapsPrincipalSockets(t *testing.T) {
	doc := runtimeconfig.Defaults()
	doc.API.SocketPath = "operator.sock"
	doc.ApplicationInterface = runtimeconfig.ApplicationInterfaceConfig{Enabled: true, SocketPath: "application.sock"}

	cfg, err := runtimeConfigFromDocument(doc)
	require.NoError(t, err)
	require.Equal(t, "operator.sock", cfg.SocketPath)
	require.True(t, cfg.ApplicationEnabled)
	require.Equal(t, "application.sock", cfg.ApplicationSocketPath)
}

func TestRuntimeConfigMapsInitialWorkloadSecurityAndLifecycleFields(t *testing.T) {
	doc := runtimeconfig.Defaults()
	doc.Workloads.AllowedPolicyRefs = []string{"trusted"}
	doc.Workloads.Initial = []runtimeconfig.WorkloadSpec{{
		ID: "worker-a", Kind: "worker", Owner: "operator", Desired: "stopped",
		Capabilities: []string{"network.read"}, PolicyRef: "trusted", RestartPolicy: "never",
	}}

	cfg, err := runtimeConfigFromDocument(doc)
	require.NoError(t, err)
	require.Len(t, cfg.Node.Workload, 1)
	require.Equal(t, []string{"network.read"}, cfg.Node.Workload[0].Capabilities)
	require.Equal(t, "trusted", cfg.Node.Workload[0].PolicyRef)
	require.Equal(t, "never", cfg.Node.Workload[0].RestartPolicy)
}

func TestLoadRuntimeConfigRejectsInvalidDocumentBeforeCreatingState(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "state")
	configPath := filepath.Join(dir, "ardents.json")
	raw := `{"api_version":"ardents.config/v2","node":{"data_dir":"` + filepath.ToSlash(dataDir) + `"}}`
	require.NoError(t, os.WriteFile(configPath, []byte(raw), 0o600))
	t.Setenv(operatorConfigFileEnv, configPath)

	_, err := loadRuntimeConfig()
	require.ErrorContains(t, err, "unsupported api_version")
	_, statErr := os.Stat(dataDir)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRestartRequiredCandidateActivatesOnNextLoad(t *testing.T) {
	dir := t.TempDir()
	doc := runtimeconfig.Defaults()
	doc.Node.DataDir = filepath.Join(dir, "data")
	configPath := writeRuntimeDocument(t, dir, doc)
	t.Setenv(operatorConfigFileEnv, configPath)

	first, err := loadRuntimeConfig()
	require.NoError(t, err)
	doc.Network.ListenPort = 24123
	writeRuntimeDocumentAt(t, configPath, doc)
	result := first.Node.OperatorConfig.Reload(context.Background())
	require.Equal(t, runtimeconfig.OutcomeRestartRequired, result.Outcome)
	require.Equal(t, 0, first.Node.Transport.ListenPort)

	restarted, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, 24123, restarted.Node.Transport.ListenPort)
}

func TestReloadRejectsUnavailableProtectedPrivacyBeforeCandidateAcceptance(t *testing.T) {
	dir := t.TempDir()
	doc := runtimeconfig.Defaults()
	doc.Node.DataDir = filepath.Join(dir, "data")
	configPath := writeRuntimeDocument(t, dir, doc)
	t.Setenv(operatorConfigFileEnv, configPath)
	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)

	doc.Privacy = runtimeconfig.PrivacyConfig{
		Required: true, CapabilityStore: "missing-store", CapabilityStoreKeyFile: "missing-key",
		ReplayKeyFile: "missing-replay-key", Subject: "p_subject",
		Discovery: runtimeconfig.PrivacyChannelConfig{Reference: "discovery", ReplayPath: "discovery-replay"},
		Data:      runtimeconfig.PrivacyChannelConfig{Reference: "data", ReplayPath: "data-replay"},
	}
	issuerPublic := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x44}, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	issuerPrincipal, err := identityprincipal.FromEd25519PublicKey(issuerPublic)
	require.NoError(t, err)
	doc.Trust.Principals = []runtimeconfig.TrustedPrincipalConfig{{
		Principal: issuerPrincipal.String(), PublicKey: base64.StdEncoding.EncodeToString(issuerPublic),
		Purposes: []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}}
	writeRuntimeDocumentAt(t, configPath, doc)
	result := cfg.Node.OperatorConfig.Reload(context.Background())
	require.Equal(t, runtimeconfig.OutcomeRejectedInvalid, result.Outcome)
	require.Equal(t, uint64(1), result.CandidateGeneration)
	require.NotContains(t, result.Reason, "missing-store")
}

func writeRuntimeDocument(t *testing.T, dir string, doc runtimeconfig.Document) string {
	t.Helper()
	path := filepath.Join(dir, "ardents.json")
	writeRuntimeDocumentAt(t, path, doc)
	return path
}

func writeRuntimeDocumentAt(t *testing.T, path string, doc runtimeconfig.Document) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

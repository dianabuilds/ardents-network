package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"

	"github.com/stretchr/testify/require"
)

func TestLoadRuntimeConfigUsesVersionedOperatorDocument(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("operator-token"), 0o600))
	configPath := filepath.Join(dir, "ardents.json")
	raw := `{
      "api_version":"ardents.config/v1",
      "node":{"name":"node-a","data_dir":"` + filepath.ToSlash(filepath.Join(dir, "data")) + `"},
      "api":{"listen_address":"127.0.0.1:19091","token_file":"` + filepath.ToSlash(tokenPath) + `"},
      "network":{"bootstrap_peers":["/ip4/10.0.0.2/tcp/60000/p2p/peer"]},
      "data":{"max_replica_bytes":4096,"desired_replicas":3,"minimum_replicas":2},
      "policy":{"disable_untrusted_route_use":true}
    }`
	require.NoError(t, os.WriteFile(configPath, []byte(raw), 0o600))
	t.Setenv(operatorConfigFileEnv, configPath)
	t.Setenv(apiTokenEnv, "")
	t.Setenv(apiTokenFileEnv, "")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, "operator-token", cfg.APIToken)
	require.Equal(t, "node-a", cfg.Node.Name)
	require.Equal(t, filepath.Join(dir, "data"), cfg.Node.Data.Dir)
	require.Equal(t, filepath.Join(dir, "data", "waku-store.db"), cfg.Node.Transport.StorePath)
	require.Equal(t, "127.0.0.1:19091", cfg.ListenAddr)
	require.Equal(t, int64(4096), cfg.Node.Data.MaxReplicaRetentionBytes)
	require.True(t, cfg.Node.Policy.DisableUntrustedRouteUse)
	require.Equal(t, []string{"/ip4/10.0.0.2/tcp/60000/p2p/peer"}, cfg.Node.Boot.Sources)
}

func TestRuntimeConfigMapsExplicitOperatorCredential(t *testing.T) {
	doc := runtimeconfig.Defaults()
	doc.API.OperatorSubject = "automation-release"
	doc.API.Capabilities = []string{"node.status", "diagnostics.health_summary"}
	doc.API.CredentialExpiresAt = "2027-01-02T03:04:05Z"

	cfg, err := runtimeConfigFromDocument(doc, "operator-token")
	require.NoError(t, err)
	require.Equal(t, "automation-release", cfg.APISubject)
	require.Equal(t, doc.API.Capabilities, cfg.APICapabilities)
	require.Equal(t, time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC), cfg.APICredentialEnd)
}

func TestRuntimeConfigMapsInitialWorkloadSecurityAndLifecycleFields(t *testing.T) {
	doc := runtimeconfig.Defaults()
	doc.Workloads.AllowedPolicyRefs = []string{"trusted"}
	doc.Workloads.Initial = []runtimeconfig.WorkloadSpec{{
		ID: "worker-a", Kind: "worker", Owner: "operator", Desired: "stopped",
		Capabilities: []string{"network.read"}, PolicyRef: "trusted", RestartPolicy: "never",
	}}

	cfg, err := runtimeConfigFromDocument(doc, "operator-token")
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

func TestOperatorDocumentEnvironmentTokenOverridesFileReference(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "ardents.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
      "api_version":"ardents.config/v1",
      "api":{"token_file":"missing"}
    }`), 0o600))
	t.Setenv(operatorConfigFileEnv, configPath)
	t.Setenv(apiTokenEnv, "environment-token")
	t.Setenv(apiTokenFileEnv, "")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)
	require.Equal(t, "environment-token", cfg.APIToken)
	require.Equal(t, "configured", cfg.Node.OperatorConfig.Snapshot().Effective["api"].(map[string]any)["token_file"])
	require.Equal(t, runtimeconfig.OutcomeUnchanged, cfg.Node.OperatorConfig.Reload(context.Background()).Outcome)
}

func TestOperatorDocumentDoesNotLeakCredentialPathOnFailure(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "private", "operator-token")
	doc := runtimeconfig.Defaults()
	doc.API.TokenFile = secretPath
	configPath := writeRuntimeDocument(t, dir, doc)
	t.Setenv(operatorConfigFileEnv, configPath)
	t.Setenv(apiTokenEnv, "")
	t.Setenv(apiTokenFileEnv, "")

	_, err := loadRuntimeConfig()
	require.EqualError(t, err, "api credential source is unavailable or invalid")
	require.NotContains(t, err.Error(), secretPath)
}

func TestRestartRequiredCandidateActivatesOnNextLoad(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("operator-token"), 0o600))
	doc := runtimeconfig.Defaults()
	doc.API.TokenFile = tokenPath
	doc.Node.DataDir = filepath.Join(dir, "data")
	configPath := writeRuntimeDocument(t, dir, doc)
	t.Setenv(operatorConfigFileEnv, configPath)
	t.Setenv(apiTokenEnv, "")
	t.Setenv(apiTokenFileEnv, "")

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
	tokenPath := filepath.Join(dir, "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("operator-token"), 0o600))
	doc := runtimeconfig.Defaults()
	doc.API.TokenFile = tokenPath
	doc.Node.DataDir = filepath.Join(dir, "data")
	configPath := writeRuntimeDocument(t, dir, doc)
	t.Setenv(operatorConfigFileEnv, configPath)
	t.Setenv(apiTokenEnv, "")
	t.Setenv(apiTokenFileEnv, "")
	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)

	doc.Privacy = runtimeconfig.PrivacyConfig{
		Required: true, CapabilityStore: "missing-store", CapabilityStoreKeyFile: "missing-key",
		ReplayKeyFile: "missing-replay-key", Subject: "p_subject",
		TrustedIssuers: map[string]string{"p_issuer": "public"},
		Discovery:      runtimeconfig.PrivacyChannelConfig{Reference: "discovery", ReplayPath: "discovery-replay"},
		Data:           runtimeconfig.PrivacyChannelConfig{Reference: "data", ReplayPath: "data-replay"},
	}
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

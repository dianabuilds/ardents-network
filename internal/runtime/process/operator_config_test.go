package process

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	hostingservice "ardents/internal/hosting/service"
	runtimeconfig "ardents/internal/runtime/config"

	"github.com/stretchr/testify/require"
)

func TestNodeReloadAppliesPolicyDiagnosticsAndRefreshInterval(t *testing.T) {
	doc := runtimeconfig.Defaults()
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	node := NewNode(Config{
		Name: "ardents", Data: DataConfig{Dir: t.TempDir()},
		Transport:      TransportConfig{BindAddress: "0.0.0.0"},
		OperatorConfig: manager,
	})
	require.NoError(t, node.policy.AllowServicePublication(hostingservice.Spec{ID: "service-a"}))

	doc.Policy.DisableServicePublication = true
	doc.Diagnostics.MaxEvents = 100
	doc.Network.DiscoveryRefreshSeconds = 5
	writeOperatorDocument(t, path, doc)
	result := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeApplied, result.Outcome)
	require.Error(t, node.policy.AllowServicePublication(hostingservice.Spec{ID: "service-a"}))
	require.Equal(t, 5*time.Second, node.cfg.DiscoveryRefreshInterval)
	require.Equal(t, uint64(2), node.GetEffectiveConfig().ActiveGeneration)
}

func writeOperatorDocument(t *testing.T, path string, doc runtimeconfig.Document) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

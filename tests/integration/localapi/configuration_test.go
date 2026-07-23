//go:build integration

package localapi_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	runtimeconfig "ardents/internal/config"
	runtimeprocess "ardents/internal/daemon"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestOperatorConfigurationCLIShowsReloadsAndRejectsInvalidCandidate(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "local-control-surface", ScenarioID: "OCI-001",
		Suite: "integration", Tags: []string{"integration", "local-control-surface", "configuration"},
		Speed: "default", Environment: "local",
	})
	doc := runtimeconfig.Defaults()
	doc.Observability.TokenFile = filepath.Join(t.TempDir(), "metrics-token")
	path := writeOperatorDocument(t, doc)
	manager, err := runtimeconfig.NewManager(path, doc)
	require.NoError(t, err)
	runtime := testkit.StartRuntime(t, runtimeprocess.Config{
		Name: "operator-config-surface", Boot: runtimeprocess.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeprocess.DataConfig{Dir: t.TempDir()}, OperatorConfig: manager,
	})
	harness := newCLIHarness(t, runtime.Runtime)

	shown := harness.run(t, "--output", "json", "config", "show")
	shownJSON := decodeConfigurationCLIJSON(t, shown.stdout)
	configuration := shownJSON["configuration"].(map[string]any)
	require.Equal(t, "1", configuration["activeGeneration"])
	effective := configuration["effective"].(map[string]any)
	require.Equal(t, "configured", effective["observability"].(map[string]any)["token_file"])
	require.NotContains(t, shown.stdout, doc.Observability.TokenFile)

	doc.Policy.DisableServicePublication = true
	writeOperatorDocumentAt(t, path, doc)
	applied := harness.run(t, "--output", "json", "config", "reload")
	appliedResult := decodeConfigurationCLIJSON(t, applied.stdout)["result"].(map[string]any)
	require.Equal(t, "applied", appliedResult["outcome"])
	require.Equal(t, "2", appliedResult["activeGeneration"])

	require.NoError(t, os.WriteFile(path, []byte(`{"api_version":"broken"}`), 0o600))
	rejected := harness.run(t, "--output", "json", "config", "reload")
	rejectedResult := decodeConfigurationCLIJSON(t, rejected.stdout)["result"].(map[string]any)
	require.Equal(t, "rejected_invalid", rejectedResult["outcome"])
	require.Equal(t, "2", rejectedResult["activeGeneration"])
}

func decodeConfigurationCLIJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &value))
	return value
}

func writeOperatorDocument(t *testing.T, doc runtimeconfig.Document) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocumentAt(t, path, doc)
	return path
}

func writeOperatorDocumentAt(t *testing.T, path string, doc runtimeconfig.Document) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

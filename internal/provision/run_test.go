package provision

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunWritesCanonicalSocketConfigurationWithoutBearerArtifacts(t *testing.T) {
	root := t.TempDir()
	authorityDir := filepath.Join(root, "authority")
	nodeDir := filepath.Join(root, "node")
	secretDir := filepath.Join(root, "secret")
	applicationDir := applicationDataDir(nodeDir)

	var output bytes.Buffer
	err := run([]string{
		"--authority-dir", authorityDir,
		"--node-dir", nodeDir,
		"--secret-dir", secretDir,
		"--node-name", "node-a",
		"--transport-port", "61000",
	}, &output, func() time.Time { return time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC) })
	require.NoError(t, err)
	require.Contains(t, output.String(), filepath.Join(secretDir, "operator.json"))

	raw, err := os.ReadFile(filepath.Join(secretDir, "operator.json"))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(raw, &document))
	api := document["api"].(map[string]any)
	application := document["application_interface"].(map[string]any)
	trust := document["trust"].(map[string]any)
	principals := trust["principals"].([]any)
	require.Equal(t, filepath.Join(secretDir, "control.sock"), api["socket_path"])
	require.Equal(t, filepath.Join(applicationDir, "application.sock"), application["socket_path"])
	require.NotContains(t, api, "token_file")
	require.NotContains(t, api, "listen_address")
	require.NotContains(t, application, "token_file")
	require.NotContains(t, application, "listen_address")
	require.Len(t, principals, 1)
	trusted := principals[0].(map[string]any)
	require.Equal(t, []any{"channel.issue"}, trusted["purposes"])
	require.NotContains(t, document["privacy"].(map[string]any), "trusted_issuers")
	require.NotContains(t, document["network"].(map[string]any), "trust_anchors")

	_, err = os.Stat(filepath.Join(secretDir, "api-token"))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(filepath.Join(applicationDir, "application-token"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	hostingservice "ardents/internal/workload/registry"

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
	require.NoError(t, node.policy.AllowServicePublication(hostingservice.ServiceSpec{ID: "service-a"}))

	doc.Policy.DisableServicePublication = true
	doc.Diagnostics.MaxEvents = 100
	doc.Network.DiscoveryRefreshSeconds = 5
	writeOperatorDocument(t, path, doc)
	result := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeApplied, result.Outcome)
	require.Error(t, node.policy.AllowServicePublication(hostingservice.ServiceSpec{ID: "service-a"}))
	require.Equal(t, 5*time.Second, node.cfg.DiscoveryRefreshInterval)
	require.Equal(t, uint64(2), node.GetEffectiveConfig().ActiveGeneration)
}

func TestNodeReloadDegradesWhenRollbackCannotRestoreRuntime(t *testing.T) {
	doc := runtimeconfig.Defaults()
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeOperatorDocument(t, path, doc)
	manager, err := runtimeconfig.NewManager(path, doc,
		configFailureApplier{failRollback: true},
		configFailureApplier{failApply: true},
	)
	require.NoError(t, err)
	node := NewNode(Config{Name: "ardents", Data: DataConfig{Dir: t.TempDir()}, OperatorConfig: manager})
	require.NoError(t, node.life.Move("starting"))
	require.NoError(t, node.life.Move("initializing"))
	require.NoError(t, node.life.Move("ready"))

	doc.Policy.DisableServicePublication = true
	writeOperatorDocument(t, path, doc)
	result := node.ReloadConfig(context.Background())

	require.Equal(t, runtimeconfig.OutcomeRollbackFailed, result.Outcome)
	require.Equal(t, "degraded", node.life.State())
	snapshot := node.Snapshot()
	require.True(t, hasSubsystemReason(snapshot.Diag.Health.Subsystems, "configuration", "config.reload.rollback_failed"))
}

type configFailureApplier struct {
	failApply    bool
	failRollback bool
}

func (configFailureApplier) Prepare(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	return nil
}

func (a configFailureApplier) Apply(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	if a.failApply {
		return errors.New("apply failed")
	}
	return nil
}

func (a configFailureApplier) Rollback(context.Context, runtimeconfig.Document) error {
	if a.failRollback {
		return errors.New("rollback failed")
	}
	return nil
}

func writeOperatorDocument(t *testing.T, path string, doc runtimeconfig.Document) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}

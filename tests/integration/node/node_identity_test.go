//go:build integration

package node_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	runtimeinfra "ardents/internal/daemon"
	db "ardents/internal/storage"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestNodeRestoresPersistentState(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()

	cfg := runtimeinfra.Config{
		Name: "test",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}
	first := testkit.NewRuntime(t, cfg).Node
	{
		err := first.Start(context.Background())
		require.NoErrorf(t, err, "start first node: %v", err)
	}
	{

		err := first.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}

	second := testkit.NewRuntime(t, cfg).Node
	{
		err := second.Start(context.Background())
		require.NoErrorf(t, err, "start second node: %v", err)
	}

	got := second.Snapshot()
	require.False(t, got.Store.Authority ==
		0, "expected restored authority state")
	require.False(t, got.Ident.Principal ==
		"", "expected persistent principal")
	require.False(t, got.Disco.Records == 0, "expected local discovery record")
	require.True(t, got.Trust.Usable, "expected usable local trust result")

}

func TestNodeRestoresIdentityAcrossRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()

	cfg := runtimeinfra.Config{
		Name: "test",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}
	first := testkit.NewRuntime(t, cfg).Node
	{
		err := first.Start(context.Background())
		require.NoErrorf(t, err, "start first node: %v", err)
	}

	firstSnap := first.Snapshot()
	{
		err := first.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}

	second := testkit.NewRuntime(t, cfg).Node
	{
		err := second.Start(context.Background())
		require.NoErrorf(t, err, "start second node: %v", err)
	}

	secondSnap := second.Snapshot()
	require.Falsef(t, firstSnap.Ident.Principal !=
		secondSnap.Ident.
			Principal, "principal changed: %q != %q", firstSnap.Ident.Principal, secondSnap.Ident.Principal)
	require.Falsef(t, firstSnap.Ident.Source !=
		"created", "first source = %q, want created", firstSnap.Ident.Source)
	require.Falsef(t, secondSnap.Ident.Source !=
		"restored", "second source = %q, want restored", secondSnap.Ident.Source)

}

func TestNodeStoresPrivateKeyOutsideGeneralState(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "node",
		ScenarioID:  "NRI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "node"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	n := testkit.NewRuntime(t, runtimeinfra.Config{
		Name: "identity-boundary",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}).Node
	{
		err := n.Start(context.Background())
		require.NoErrorf(t, err, "start node: %v", err)
	}
	var persisted map[string]any
	found, err := db.LoadJSON(db.PathInDir(dir), "node-runtime", "state", &persisted)
	require.NoErrorf(t, err, "load node runtime state: %v", err)
	require.True(t, found, "expected node runtime state to be persisted")
	stateRaw, err := json.Marshal(persisted)
	require.NoErrorf(t, err, "encode node runtime state: %v", err)
	require.NotContains(t, string(stateRaw), "legacy_private_key")
	require.NotContains(t, string(stateRaw), "private_key")

	raw, err := os.ReadFile(filepath.Join(dir, "identity_key.json"))
	require.NoErrorf(t, err, "read identity keystore: %v", err)

	var stored map[string]string
	{
		err := json.Unmarshal(raw, &stored)
		require.NoErrorf(t, err, "decode identity keystore: %v", err)
	}
	require.False(t, stored["private_key"] ==
		"", "expected private key in dedicated keystore")

}

func TestNodeRestoresStoppedDataDirectoryBackup(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "node", ScenarioID: "NRI-001",
		Suite: "integration", Tags: []string{"integration", "node", "backup"},
		Speed: "default", Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	sourceDir := t.TempDir()
	cfg := runtimeinfra.Config{
		Name: "backup-source", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
	}
	first := testkit.NewRuntime(t, cfg).Node
	require.NoError(t, first.Start(context.Background()))
	want := first.Snapshot().Ident
	require.NoError(t, first.Stop(context.Background()))

	restoreDir := t.TempDir()
	require.NoError(t, copyStoppedBackup(restoreDir, sourceDir))
	_, found, err := db.ReadStrictPrivateFileBounded(filepath.Join(restoreDir, "identity_key.json"), 1<<20)
	require.NoError(t, err)
	require.True(t, found, "restored identity key must remain private")
	cfg.Data.Dir = restoreDir
	restored := testkit.NewRuntime(t, cfg).Node
	require.NoError(t, restored.Start(context.Background()))
	got := restored.Snapshot().Ident
	require.Equal(t, want.Principal, got.Principal)
	require.Equal(t, "restored", got.Source)
	require.NoError(t, restored.Stop(context.Background()))
}

func TestNodeRejectsPartialIdentityRestore(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "node", ScenarioID: "NRI-001",
		Suite: "integration", Tags: []string{"integration", "node", "backup"},
		Speed: "default", Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	dir := t.TempDir()
	cfg := runtimeinfra.Config{
		Name: "partial-restore", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	}
	first := testkit.NewRuntime(t, cfg).Node
	require.NoError(t, first.Start(context.Background()))
	want := first.Snapshot().Ident.Principal
	require.NoError(t, first.Stop(context.Background()))
	require.NoError(t, os.Remove(filepath.Join(dir, "identity_key.json")))

	broken := testkit.NewRuntime(t, cfg).Node
	err := broken.Start(context.Background())
	require.ErrorContains(t, err, "restore matching state and key backup")
	var persisted struct {
		Identity struct {
			Principal string `json:"principal"`
		} `json:"identity"`
	}
	found, loadErr := db.LoadJSON(db.PathInDir(dir), "node-runtime", "state", &persisted)
	require.NoError(t, loadErr)
	require.True(t, found)
	require.Equal(t, want, persisted.Identity.Principal)
}

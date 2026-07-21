package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCatalogFileExtractsScenarioMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample_test.go")
	source := `package sample

import (
	"testing"

	"ardents/tests/testkit"
)

func TestSampleScenario(t *testing.T) {
	_ = testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration,
		Domain: "network-foundation",
		ScenarioID: "NFI-001",
		Suite: "integration",
		Tags: []string{"integration", "network"},
		Speed: "default",
		Environment: "local",
	})
}
`
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	entries, err := parseCatalogFile(path, "ardents/tests/sample")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "TestSampleScenario", entries[0].TestName)
	require.Equal(t, "network-foundation", entries[0].Domain)
	require.Equal(t, "NFI-001", entries[0].ScenarioID)
	require.Equal(t, []string{"integration", "network"}, entries[0].Tags)
}

func TestFilterCatalogMatchesTagAndScenario(t *testing.T) {
	entries := []catalogEntry{
		{ScenarioID: "NFI-001", Domain: "network-foundation", Layer: "integration", Suite: "integration", Tags: []string{"network"}},
		{ScenarioID: "WKI-001", Domain: "workload", Layer: "integration", Suite: "integration", Tags: []string{"workload"}},
	}

	filtered := filterCatalog(entries, "integration", "network-foundation", "NFI-001", "network", "integration")
	require.Len(t, filtered, 1)
	require.Equal(t, "NFI-001", filtered[0].ScenarioID)
}

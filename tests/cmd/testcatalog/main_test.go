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

func TestParseScenarioDocExtractsScenarioAndRelatedTests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scenario.md")
	source := `## Scenario ID

` + "`DKI-001`" + `

## Layer

` + "`integration`" + `

## Domain

` + "`Discovery`" + `

## Related Tests

- ` + "`tests/integration/discovery/domain_test.go::TestDiscoveryPublishesNodeEnvelope`" + `

## False Positive Risk

The assertion could observe only local state.

## False Negative Risk

Bounded convergence may exceed the test deadline.
`
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	doc, err := parseScenarioDoc(path)
	require.NoError(t, err)
	require.Equal(t, "DKI-001", doc.ScenarioID)
	require.Equal(t, "integration", doc.Layer)
	require.Equal(t, "Discovery", doc.Domain)
	require.Equal(t, []string{"tests/integration/discovery/domain_test.go::TestDiscoveryPublishesNodeEnvelope"}, doc.RelatedTests)
	require.True(t, doc.FalsePositiveRisk)
	require.True(t, doc.FalseNegativeRisk)
}

func TestInventoryScenarioEntryRequiresRiskAnalysisAndCanonicalMetadata(t *testing.T) {
	entry := inventoryScenarioEntry(scenarioDoc{ScenarioID: "BAD-001", Layer: "Integration."}, nil)
	require.Contains(t, entry.Issues, "scenario doc layer must be integration or e2e")
	require.Contains(t, entry.Issues, "scenario doc is missing domain")
	require.Contains(t, entry.Issues, "scenario doc is missing non-empty False Positive Risk")
	require.Contains(t, entry.Issues, "scenario doc is missing non-empty False Negative Risk")
}

func TestParseScenarioDocAcceptsInlineRiskFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "inline.md")
	source := "- Scenario ID: `INLINE-001`\n- Layer: `e2e`\n- Domain: `node`\n" +
		"- `False Positive Risk`:\n  runtime truth is not asserted\n" +
		"- `False Negative Risk`:\n  bounded convergence may be slow\n"
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	doc, err := parseScenarioDoc(path)
	require.NoError(t, err)
	require.True(t, doc.FalsePositiveRisk)
	require.True(t, doc.FalseNegativeRisk)
}

func TestParseInventoryFileDetectsFormalAndMissingBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample_test.go")
	source := `package sample

import (
	"testing"

	"ardents/tests/testkit"
)

func TestFormalScenario(t *testing.T) {
	_ = testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration,
		Domain: "network-foundation",
		ScenarioID: "NFI-001",
		Suite: "integration",
	})
}

func TestMissingScenario(t *testing.T) {}
func TestProcessHelper(t *testing.T) {}
`
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))

	tests, err := parseInventorySource(path, source, "ardents")
	require.NoError(t, err)
	require.Len(t, tests, 2)
	require.Equal(t, "formal", tests[0].BindingSource)
	require.Equal(t, "NFI-001", tests[0].ScenarioID)
	require.Equal(t, "missing", tests[1].BindingSource)
}

func TestHelperProcessTestNameConvention(t *testing.T) {
	require.True(t, isHelperProcessTest("TestReadinessWorkloadHelper"))
	require.False(t, isHelperProcessTest("TestWorkloadRecovery"))
}

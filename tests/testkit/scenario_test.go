package testkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecValidateRequiresMandatoryFields(t *testing.T) {
	err := Spec{}.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "layer")

	err = (Spec{Layer: LayerIntegration, Domain: "network"}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "scenario id")
}

func TestBeginScenarioWritesReportFile(t *testing.T) {
	dir := t.TempDir()
	t.Run("scenario", func(t *testing.T) {
		t.Setenv(ReportDirEnv, dir)

		scenario := BeginScenario(t, Spec{
			Layer:       LayerIntegration,
			Domain:      "network-foundation",
			ScenarioID:  "NFI-001",
			Suite:       "integration",
			Tags:        []string{"integration"},
			Speed:       "default",
			Environment: "local",
		})

		scenario.Precondition("bootstrap", func(t *testing.T) {})
		scenario.Step("join relay", func(t *testing.T) {})
		scenario.Assert("report captured", func(t *testing.T) {})
	})

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	payload, err := os.ReadFile(files[0])
	require.NoError(t, err)

	var report Report
	require.NoError(t, json.Unmarshal(payload, &report))
	require.Equal(t, "NFI-001", report.Spec.ScenarioID)
	require.Equal(t, "passed", report.Status)
	require.Len(t, report.Steps, 3)
}

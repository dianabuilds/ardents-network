package routeexperiment

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type experimentReceipt struct {
	SchemaVersion       string `json:"schema_version"`
	Status              string `json:"status"`
	Decision            string `json:"decision"`
	InputManifestSHA256 string `json:"input_manifest_sha256"`
	ExperimentSHA256    string `json:"experiment_sha256"`
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 32*1024*1024 {
		return errors.New("route-experiment evidence exceeds 32 MiB")
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func copyEvidenceFile(source, target string) error {
	info, err := os.Stat(source)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 32*1024*1024 {
		return errors.New("condition evidence is missing or oversized")
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o600)
}

func writeFinalEvidence(directory string, summary experimentSummary) error {
	experimentPath := filepath.Join(directory, "experiment.json")
	if err := writeJSON(experimentPath, summary); err != nil {
		return err
	}
	manifestPath := filepath.Join(directory, "input-manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return nil
	}
	manifestDigest, err := hashFile(manifestPath)
	if err != nil {
		return err
	}
	experimentDigest, err := hashFile(experimentPath)
	if err != nil {
		return err
	}
	receipt := experimentReceipt{
		SchemaVersion: "carrier-lab-route-experiment-receipt/v1", Status: summary.Status, Decision: summary.Decision,
		InputManifestSHA256: manifestDigest, ExperimentSHA256: experimentDigest,
	}
	if err := writeJSON(filepath.Join(directory, "experiment-verdict.json"), receipt); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "report.md"), []byte(humanReport(summary, receipt)), 0o600)
}

func humanReport(summary experimentSummary, receipt experimentReceipt) string {
	var report strings.Builder
	fmt.Fprintf(&report, "# Carrier Lab R-013 result\n\nStatus: **%s**  \nDecision: **%s**  \nRun: `%s`\n\n", summary.Status, summary.Decision, summary.RunID)
	for _, name := range []string{"direct", "c3", "c5-c2"} {
		condition, found := summary.Conditions[name]
		if !found {
			continue
		}
		setupTimes := setupDurations(condition.Setups)
		fmt.Fprintf(&report, "## %s\n\n- Setup: %d/%d; p50 %.3fs; p95 %.3fs.\n", name, successfulSetups(condition.Setups), len(condition.Setups), nearestRank(setupTimes, 50).Seconds(), nearestRank(setupTimes, 95).Seconds())
		for _, direction := range []string{directionUpload, directionDownload} {
			rates, complete := verifiedRates(condition.Streams, direction)
			if complete {
				fmt.Fprintf(&report, "- %s: min %.2f Mbit/s; median %.2f Mbit/s.\n", direction, minimum(rates)/1_000_000, median(rates)/1_000_000)
			}
		}
		fmt.Fprintf(&report, "- Cleanup: %t.\n\n", condition.CleanupPassed)
	}
	if summary.Reference != nil {
		fmt.Fprintf(&report, "## Tor/Chutney reference\n\nStatus: %s; Tor: `%s`; flavor: `%s`; isolated network namespace: %t.\n\n", summary.Reference.Status, summary.Reference.TorVersion, summary.Reference.NetworkFlavor, summary.Reference.OfflineNamespace)
	}
	if len(summary.Failures) > 0 {
		report.WriteString("## Failed gates\n\n")
		for _, failure := range summary.Failures {
			fmt.Fprintf(&report, "- %s\n", failure)
		}
		report.WriteString("\n")
	}
	fmt.Fprintf(&report, "## Evidence binding\n\n- Input manifest SHA-256: `%s`\n- Experiment SHA-256: `%s`\n", receipt.InputManifestSHA256, receipt.ExperimentSHA256)
	return report.String()
}

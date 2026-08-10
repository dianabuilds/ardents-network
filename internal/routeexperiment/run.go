package routeexperiment

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

const experimentSchema = "carrier-lab-route-experiment/v1"

type experimentSummary struct {
	SchemaVersion string                     `json:"schema_version"`
	RunID         string                     `json:"run_id"`
	Status        string                     `json:"status"`
	Decision      string                     `json:"decision"`
	Conditions    map[string]conditionResult `json:"conditions"`
	Negatives     negativeResult             `json:"negatives"`
	Failures      []string                   `json:"failures,omitempty"`
	Reference     *referenceResult           `json:"tor_reference,omitempty"`
	StartedUTC    string                     `json:"started_utc"`
	FinishedUTC   string                     `json:"finished_utc"`
}

// Run owns the complete frozen R-013 sequence. It consumes immutable image IDs
// and pre-prepared external reference inputs; it never builds or downloads.
func Run(ctx context.Context, identity preflight.RunLayout, applicationImage, toolImage, referenceDirectory string) (evidenceDirectory string, runErr error) {
	runID, repositoryRoot, runDirectory, evidenceDirectory, err := identity.OwnedPaths(false, false)
	if err != nil {
		return "", err
	}
	for _, directory := range []string{runDirectory, evidenceDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return evidenceDirectory, err
		}
	}
	summary := experimentSummary{
		SchemaVersion: experimentSchema, RunID: runID, Status: "failed", Decision: "invalid",
		Conditions: make(map[string]conditionResult, 4), StartedUTC: time.Now().UTC().Format(time.RFC3339Nano),
	}
	defer func() {
		summary.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
		cleanupErr := removeExperimentRun(runDirectory, evidenceDirectory)
		if cleanupErr != nil {
			summary.Failures = append(summary.Failures, cleanupErr.Error())
			runErr = errors.Join(runErr, cleanupErr)
		}
		evidenceErr := writeFinalEvidence(evidenceDirectory, summary)
		runErr = errors.Join(runErr, evidenceErr)
	}()
	manifest, err := prepareManifest(ctx, runID, repositoryRoot, applicationImage, toolImage, referenceDirectory)
	if err != nil {
		return evidenceDirectory, err
	}
	if err := writeJSON(filepath.Join(evidenceDirectory, "input-manifest.json"), manifest); err != nil {
		return evidenceDirectory, err
	}
	return runExperiment(ctx, identity, manifest, &summary)
}

func runExperiment(ctx context.Context, identity preflight.RunLayout, manifest inputManifest, summary *experimentSummary) (string, error) {
	_, _, _, evidenceDirectory, _ := identity.OwnedPaths(true, true)
	direct, err := runNativeCondition(ctx, identity, manifest, "direct")
	if err != nil {
		return evidenceDirectory, err
	}
	summary.Conditions["direct"] = direct
	c3, err := runNativeCondition(ctx, identity, manifest, "c3")
	if err != nil {
		return evidenceDirectory, err
	}
	summary.Conditions["c3"] = c3
	candidate, err := runNativeCondition(ctx, identity, manifest, "c5-c2")
	if err != nil {
		return evidenceDirectory, err
	}
	summary.Conditions["c5-c2"] = candidate
	var referenceFailure string
	if successfulSetups(candidate.Setups) >= 19 {
		reference, referenceErr := runTorReference(ctx, identity, manifest)
		summary.Reference = &reference
		if referenceErr != nil {
			referenceFailure = referenceErr.Error()
		}
	}
	negatives, negativeErr := runNegativeSuite(ctx, identity, manifest)
	summary.Negatives = negatives
	verdict := evaluateCandidate(direct, candidate, negatives)
	summary.Status, summary.Decision, summary.Failures = "completed", verdict.Decision, verdict.Failures
	if referenceFailure != "" {
		summary.Failures = append(summary.Failures, referenceFailure)
		summary.Status, summary.Decision = "invalid", "invalid"
		return evidenceDirectory, errors.New(referenceFailure)
	}
	if negativeErr != nil && len(summary.Failures) == 0 {
		return evidenceDirectory, negativeErr
	}
	return evidenceDirectory, nil
}

func successfulSetups(samples []setupSample) int {
	passed := 0
	for _, sample := range samples {
		if sample.Passed {
			passed++
		}
	}
	return passed
}

func removeExperimentRun(runDirectory, evidenceDirectory string) error {
	if filepath.Dir(runDirectory) != filepath.Dir(evidenceDirectory) || runDirectory == evidenceDirectory || !filepath.IsAbs(runDirectory) {
		return errors.New("refusing to clean an unowned route-experiment path")
	}
	if err := os.RemoveAll(runDirectory); err != nil {
		return err
	}
	if _, err := os.Stat(runDirectory); !os.IsNotExist(err) {
		return errors.New("route-experiment run directory remains after cleanup")
	}
	return nil
}

package routecomparison

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/lab/nativecircuit"
	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
)

type failureSummary struct {
	Status              string          `json:"status"`
	Checks              map[string]bool `json:"checks"`
	FailureMilliseconds int64           `json:"failure_milliseconds"`
}

func runNegativeSuite(ctx context.Context, identity preflight.RunLayout, manifest inputManifest) (negativeResult, error) {
	result := negativeResult{}
	var runErr error
	for _, name := range []string{"wrong-instance", "modified-record", "replay", "wrong-binding", "oversized-frame", "invalid-state"} {
		command := exec.CommandContext(ctx, "docker", "run", "--rm", "--network", "none", manifest.ApplicationImage, "native-negative", "--case", name)
		output, err := command.CombinedOutput()
		passed := err == nil && len(output) > 0 && len(output) < 64*1024
		result.Cases = append(result.Cases, negativeCase{Name: name, Passed: passed})
		if !passed {
			runErr = errors.Join(runErr, errors.New("negative case did not fail closed: "+name))
		}
	}
	failure, err := runRendezvousFailure(ctx, identity, manifest)
	result.Cases = append(result.Cases, negativeCase{Name: "rendezvous-process", Passed: err == nil})
	result.FailureElapsed = time.Duration(failure.FailureMilliseconds) * time.Millisecond
	return result, errors.Join(runErr, err)
}

func runRendezvousFailure(ctx context.Context, identity preflight.RunLayout, manifest inputManifest) (failureSummary, error) {
	runID, repositoryRoot, runDirectory, retainedRoot, err := identity.OwnedPaths(true, true)
	if err != nil {
		return failureSummary{}, err
	}
	childID := runID + "-rendezvousfailure"
	parentSession := filepath.Dir(runDirectory)
	childSession := filepath.Join(parentSession, "ardents-experiment-session."+childID)
	if err := os.Mkdir(childSession, 0o700); err != nil {
		return failureSummary{}, err
	}
	childLayout, err := preflight.NewRunLayout(childSession, repositoryRoot, parentSession, childID)
	if err != nil {
		return failureSummary{}, err
	}
	childEvidence, runErr := nativecircuit.Run(ctx, childLayout, manifest.ApplicationImage, manifest.ToolImage, "rendezvous-process")
	retained := filepath.Join(retainedRoot, "negatives", "rendezvous-process")
	copyErr := copyEvidenceTree(childEvidence, retained)
	var summary failureSummary
	parseErr := readJSON(filepath.Join(retained, "native-run.json"), &summary)
	cleanupErr := removeChildSession(childSession, parentSession)
	if summary.Status != "passed" || !summary.Checks["failure_within_15_seconds"] || !summary.Checks["cleanup_complete"] {
		parseErr = errors.Join(parseErr, errors.New("rendezvous process failure contract did not pass"))
	}
	return summary, errors.Join(runErr, copyErr, parseErr, cleanupErr)
}

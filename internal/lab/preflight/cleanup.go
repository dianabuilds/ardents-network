package preflight

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FinalizeCleanup removes resources owned by one preflight run, proves that
// repeated cleanup is harmless, and writes the final verdict.
func FinalizeCleanup(layout RunLayout, resources OwnedResources) (Result, error) {
	if err := layout.validateOwnedPaths(false, true); err != nil {
		return Result{}, fmt.Errorf("validate run layout before cleanup: %w", err)
	}
	record, err := readManifest(layout)
	if err != nil {
		return Result{}, err
	}
	if record.RunID != layout.runID {
		return Result{}, errors.New("manifest run ID does not match the run layout")
	}
	first, firstErr := cleanupRunDirectory(layout, record.RunID)
	second, secondErr := cleanupRunDirectory(layout, record.RunID)
	cleanup := cleanupState{
		OwnedContainersAbsent:        resources.ContainersAbsent,
		OwnedNetworksAbsent:          resources.NetworksAbsent,
		OwnedVolumesAbsent:           resources.VolumesAbsent,
		TemporaryCredentialsRemoved:  firstErr == nil && (first.Removed || first.AlreadyAbsent),
		GoCachesRemoved:              firstErr == nil && (first.Removed || first.AlreadyAbsent),
		RunDirectoryRemoved:          firstErr == nil && (first.Removed || first.AlreadyAbsent),
		RepeatedCleanupAlreadyAbsent: secondErr == nil && second.AlreadyAbsent,
		EvidenceRetained:             true,
	}
	report := cleanupReport{
		SchemaVersion: cleanupSchemaVersion,
		RunID:         layout.runID,
		Status:        statusPassed,
		Cleanup:       cleanup,
	}
	if firstErr != nil || secondErr != nil || !resources.ContainersAbsent ||
		!resources.NetworksAbsent || !resources.VolumesAbsent ||
		!cleanup.RunDirectoryRemoved || !cleanup.RepeatedCleanupAlreadyAbsent {
		report.Status = statusPreflightFailed
		report.Failure = strings.TrimSpace(strings.Join([]string{errorText(firstErr), errorText(secondErr)}, " "))
		record.FailureReasons = append(record.FailureReasons, failure(
			reasonCleanupFailure,
			stageResourceCleanup,
			"outer cleanup did not remove every owned resource",
		))
	}
	record.Cleanup = &cleanup
	if record.Status == statusChecksPassed && report.Status == statusPassed {
		record.Status = statusPassed
	} else {
		record.Status = statusPreflightFailed
	}
	if err := writeEvidence(layout, record, report); err != nil {
		return Result{}, err
	}
	return Result{Status: record.Status, ChecksPassed: record.Status == statusPassed, CleanupRequired: record.Status != statusPassed, Passed: record.Status == statusPassed}, nil
}

func cleanupRunDirectory(layout RunLayout, manifestRunID string) (cleanupAttempt, error) {
	if manifestRunID != layout.runID {
		return cleanupAttempt{}, errors.New("manifest run ID does not match the run layout")
	}
	if err := layout.validateOwnedPaths(false, true); err != nil {
		return cleanupAttempt{}, err
	}
	_, err := os.Stat(layout.runDir)
	if os.IsNotExist(err) {
		return cleanupAttempt{AlreadyAbsent: true}, nil
	}
	if err != nil {
		return cleanupAttempt{}, err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return cleanupAttempt{}, err
	}
	return cleanupAttempt{Removed: true}, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func pathWithin(path, parent string) (bool, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	absoluteParent, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(absoluteParent, absolutePath)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func samePath(left, right string) bool {
	absLeft, errLeft := filepath.Abs(left)
	absRight, errRight := filepath.Abs(right)
	return errLeft == nil && errRight == nil && filepath.Clean(absLeft) == filepath.Clean(absRight)
}

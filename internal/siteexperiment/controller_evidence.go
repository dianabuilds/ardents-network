package siteexperiment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
)

func validImageID(value string) bool {
	algorithm, digest, found := strings.Cut(value, ":")
	decoded, err := hex.DecodeString(digest)
	return found && algorithm == "sha256" && err == nil && len(decoded) == sha256.Size
}

func writeBoundedJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 4*1024*1024 {
		return errors.New("gate C evidence exceeds 4 MiB")
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func writeAttemptCleanup(retained string, sequence int) error {
	return writeBoundedJSON(filepath.Join(retained, "attempts", formatAttempt(sequence), "cleanup.json"), map[string]any{
		"schema_version": "gatec-attempt-cleanup/v1", "reference_resources_removed": true,
	})
}

func writeReferenceOnlyAttemptCleanup(retained string, sequence int) error {
	return writeBoundedJSON(filepath.Join(retained, "attempts", formatAttempt(sequence), "cleanup.json"), map[string]any{
		"schema_version": "gatec-attempt-cleanup/v1", "reference_resources_removed": true, "native_route_not_started": true,
	})
}

func attemptCleanupProven(retained string, sequence int) bool {
	directory := filepath.Join(retained, "attempts", formatAttempt(sequence))
	var cleanup struct {
		Removed          bool `json:"reference_resources_removed"`
		NativeNotStarted bool `json:"native_route_not_started"`
	}
	var native struct {
		Checks map[string]bool `json:"checks"`
	}
	if readStrictEvidence(filepath.Join(directory, "cleanup.json"), &cleanup) != nil || !cleanup.Removed {
		return false
	}
	return cleanup.NativeNotStarted || readStrictEvidence(filepath.Join(directory, "native-run.json"), &native) == nil && native.Checks["cleanup_complete"]
}

func cleanupGateCRuntime(identity experimentrun.Layout, runDirectory string) error {
	_, _, verifiedRun, _, err := identity.OwnedPaths(true, true)
	if err != nil || verifiedRun != runDirectory {
		return errors.New("gate C cleanup identity changed")
	}
	info, err := os.Lstat(runDirectory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || filepath.Base(runDirectory) == "" {
		return errors.New("gate C runtime is not an owned real directory")
	}
	if err := os.RemoveAll(runDirectory); err != nil {
		return err
	}
	if _, err := os.Stat(runDirectory); !os.IsNotExist(err) {
		return errors.New("gate C runtime remains after cleanup")
	}
	return nil
}

func cleanupGateCPreparation(identity experimentrun.Layout) error {
	_, _, runDirectory, evidenceDirectory, err := identity.OwnedPaths(false, false)
	if err != nil {
		return err
	}
	for _, target := range []string{runDirectory, evidenceDirectory} {
		info, inspectErr := os.Lstat(target)
		if os.IsNotExist(inspectErr) {
			continue
		}
		if inspectErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("partial Gate C preparation is not an owned real directory")
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}

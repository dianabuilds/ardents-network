package routeexperiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/nativecircuit"
	"github.com/dianabuilds/ardents-network/internal/preflight"
)

type nativeAttemptSummary struct {
	Status            string          `json:"status"`
	SetupMilliseconds int64           `json:"setup_milliseconds"`
	Checks            map[string]bool `json:"checks"`
}

type nativeRoleEvidence struct {
	ApplicationBytesVerified  bool  `json:"application_bytes_verified"`
	ApplicationBytes          int   `json:"application_bytes"`
	QueueHighWaterBytes       int   `json:"queue_high_water_bytes"`
	StreamElapsedMilliseconds int64 `json:"stream_elapsed_milliseconds"`
}

type nativeResourceEvidence struct {
	Role     string  `json:"role"`
	CPUCores float64 `json:"cpu_cores"`
	RSSBytes uint64  `json:"rss_bytes"`
}

type nativeToolEvidence struct {
	Links map[string]struct {
		WireBytes uint64 `json:"wire_bytes"`
	} `json:"links"`
}

type nativeAttemptEvidence struct {
	summary   nativeAttemptSummary
	user      nativeRoleEvidence
	service   nativeRoleEvidence
	resources []nativeResourceEvidence
	queues    map[string]uint64
	linkBytes map[string]uint64
}

func runNativeAttempt(ctx context.Context, identity preflight.RunLayout, manifest inputManifest, profile string, workload manifestWorkload) (nativeAttemptEvidence, error) {
	runID, repositoryRoot, runDirectory, retainedRoot, err := identity.OwnedPaths(true, true)
	if err != nil {
		return nativeAttemptEvidence{}, err
	}
	workloadPath := filepath.Join(runDirectory, profile+"-"+workload.Name+".json")
	input := map[string]any{
		"schema_version": "carrier-lab-native-workload/v1", "profile": profile,
		"kind": workload.Kind, "seed": workload.Seed,
	}
	if workload.Kind == "stream" {
		input["direction"], input["duration_seconds"] = workload.Direction, workload.DurationSeconds
	}
	if err := writeJSON(workloadPath, input); err != nil {
		return nativeAttemptEvidence{}, err
	}
	childID := runID + "-" + compactName(profile) + "-" + compactName(workload.Name)
	parentSession := filepath.Dir(runDirectory)
	childSession := filepath.Join(parentSession, "ardents-experiment-session."+childID)
	if err := os.Mkdir(childSession, 0o700); err != nil {
		return nativeAttemptEvidence{}, err
	}
	childLayout, err := preflight.NewRunLayout(childSession, repositoryRoot, parentSession, childID)
	if err != nil {
		return nativeAttemptEvidence{}, err
	}
	childEvidence, runErr := nativecircuit.RunWorkload(ctx, childLayout, manifest.ApplicationImage, manifest.ToolImage, workloadPath)
	retained := filepath.Join(retainedRoot, "conditions", profile, workload.Name)
	copyErr := copyEvidenceTree(childEvidence, retained)
	parsePath := childEvidence
	if copyErr == nil {
		parsePath = retained
	}
	attempt, parseErr := readNativeAttempt(parsePath)
	cleanupErr := removeChildSession(childSession, parentSession)
	return attempt, errors.Join(runErr, copyErr, parseErr, cleanupErr)
}

func readNativeAttempt(directory string) (nativeAttemptEvidence, error) {
	result := nativeAttemptEvidence{queues: make(map[string]uint64), linkBytes: make(map[string]uint64)}
	if err := readJSON(filepath.Join(directory, "native-run.json"), &result.summary); err != nil {
		return result, err
	}
	roles, err := os.ReadDir(filepath.Join(directory, "native-roles"))
	if err != nil {
		return result, err
	}
	for _, entry := range roles {
		var evidence nativeRoleEvidence
		if err := readJSON(filepath.Join(directory, "native-roles", entry.Name()), &evidence); err != nil {
			return result, err
		}
		role := strings.TrimSuffix(entry.Name(), ".json")
		result.queues[role] = uint64(evidence.QueueHighWaterBytes)
		if role == "user" {
			result.user = evidence
		} else if role == "service" {
			result.service = evidence
		}
	}
	resourcePath := filepath.Join(directory, "resource-samples.json")
	if _, err := os.Stat(resourcePath); err == nil {
		if err := readJSON(resourcePath, &result.resources); err != nil {
			return result, err
		}
	}
	tools, err := os.ReadDir(filepath.Join(directory, "native-tools"))
	if err != nil {
		return result, err
	}
	for _, entry := range tools {
		if !strings.HasPrefix(entry.Name(), "capture-") {
			continue
		}
		var evidence nativeToolEvidence
		if err := readJSON(filepath.Join(directory, "native-tools", entry.Name()), &evidence); err != nil {
			return result, err
		}
		for link, value := range evidence.Links {
			result.linkBytes[link] = value.WireBytes
		}
	}
	return result, nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read condition evidence %s: %w", filepath.Base(path), err)
	}
	if len(data) == 0 || len(data) > 32*1024*1024 {
		return errors.New("condition evidence is empty or oversized")
	}
	return json.Unmarshal(data, target)
}

func copyEvidenceTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || strings.HasPrefix(relative, "..") || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("condition evidence tree is unsafe")
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		return copyEvidenceFile(path, destination)
	})
}

func removeChildSession(child, parent string) error {
	if filepath.Dir(child) != parent || !strings.HasPrefix(filepath.Base(child), "ardents-experiment-session.") {
		return errors.New("refusing to remove an unowned condition session")
	}
	return os.RemoveAll(child)
}

func compactName(value string) string { return strings.ReplaceAll(value, "-", "") }

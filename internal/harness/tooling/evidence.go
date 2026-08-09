package tooling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

func finishToolingSmoke(layout runLayout, project string, environment []string, capturePath string, summary *toolingSmokeSummary, runErr error) error {
	return finishToolingSmokeWithCheck(layout, project, environment, capturePath, summary, runErr, resourcesRemain)
}

func finishToolingSmokeWithCheck(layout runLayout, project string, environment []string, capturePath string, summary *toolingSmokeSummary, runErr error, projectResourcesRemain func(context.Context, string) bool) error {
	cleanupContext, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var cleanupErr error
	if len(environment) > 0 {
		_, cleanupErr = toolingComposeCommand(cleanupContext, layout, project, environment, "down", "--volumes", "--remove-orphans", "--timeout", "5")
	}
	if projectResourcesRemain(cleanupContext, project) {
		cleanupErr = errors.Join(cleanupErr, removeProjectContainers(cleanupContext, project))
		if len(environment) > 0 {
			_, _ = toolingComposeCommand(cleanupContext, layout, project, environment, "down", "--volumes", "--remove-orphans", "--timeout", "5")
		}
		if projectResourcesRemain(cleanupContext, project) {
			cleanupErr = errors.Join(cleanupErr, errors.New("tooling Compose resources remain after repeated cleanup"))
		}
	}
	if err := removeRawCapture(capturePath, filepath.Dir(capturePath)); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	} else {
		summary.Checks["raw_capture_removed"] = true
	}
	if err := removeSmokeRunDirectory(layout); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if cleanupErr == nil {
		summary.Checks["cleanup_complete"] = true
	}
	finalErr := errors.Join(runErr, cleanupErr)
	if finalErr == nil && toolingChecksPassed(summary.Checks) {
		summary.Status = "passed"
	} else {
		summary.Status = "failed"
		if finalErr == nil {
			finalErr = errors.New("one or more tooling smoke checks failed")
		}
		summary.Failure = sanitizeToolingFailure(layout, finalErr.Error())
	}
	evidenceErr := writeToolingEvidence(layout, summary)
	return errors.Join(finalErr, evidenceErr)
}

var ipv4Address = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

func sanitizeToolingFailure(layout runLayout, message string) string {
	replacements := []struct{ value, label string }{
		{layout.evidenceDir, "<evidence-dir>"},
		{layout.runDir, "<run-dir>"},
		{layout.repositoryRoot, "<repository-root>"},
	}
	slices.SortFunc(replacements, func(left, right struct{ value, label string }) int {
		return len(right.value) - len(left.value)
	})
	for _, replacement := range replacements {
		for _, value := range []string{replacement.value, filepath.ToSlash(replacement.value)} {
			if value != "" {
				message = strings.ReplaceAll(message, value, replacement.label)
			}
		}
	}
	return ipv4Address.ReplaceAllString(message, "<address>")
}

func removeProjectContainers(ctx context.Context, project string) error {
	output, err := exec.CommandContext(ctx, "docker", "ps", "--all", "--quiet", "--filter", "label=com.docker.compose.project="+project).Output()
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, containerID := range strings.Fields(string(output)) {
		if removeOutput, err := exec.CommandContext(ctx, "docker", "container", "rm", "--force", containerID).CombinedOutput(); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove owned tooling container: %w: %s", err, strings.TrimSpace(string(removeOutput))))
		}
	}
	return cleanupErr
}

func writeToolingEvidence(layout runLayout, summary *toolingSmokeSummary) error {
	manifestPath := filepath.Join(layout.evidenceDir, toolingManifestFile)
	if err := writeBoundedJSON(manifestPath, summary); err != nil {
		return err
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(manifestData)
	manifestSHA256 := hex.EncodeToString(digest[:])
	verdict := toolingSmokeVerdict{SchemaVersion: toolingVerdictSchema, RunID: summary.RunID, Status: summary.Status, ManifestSHA256: manifestSHA256}
	if err := writeBoundedJSON(filepath.Join(layout.evidenceDir, toolingVerdictFile), verdict); err != nil {
		return err
	}
	return verifyToolingManifest(manifestPath, manifestSHA256)
}

func verifyToolingManifest(path, expectedSHA256 string) error {
	observed, err := hashRegularFile(path)
	if err != nil {
		return err
	}
	if observed != expectedSHA256 {
		return errors.New("tooling manifest no longer matches its verdict digest")
	}
	return nil
}

func writeControlFile(layout runLayout, name string) error {
	if name != "start" && name != "stop" {
		return errors.New("invalid tooling control file")
	}
	return os.WriteFile(filepath.Join(layout.runDir, "control", name), []byte("go\n"), 0o600)
}

func exactCapability(values []string, wanted string) bool {
	return len(values) == 1 && strings.EqualFold(strings.TrimPrefix(values[0], "CAP_"), wanted)
}

func validSHA256String(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func toolingPathAbsent(path string) bool {
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func finishSmoke(layout runLayout, environment []string, summary *composeSmokeSummary, runErr error) error {
	cleanupContext, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	project := composeProjectName(layout.runID)
	var cleanupErr error
	if len(environment) > 0 {
		_, cleanupErr = composeCommand(cleanupContext, layout, project, environment, "down", "--volumes", "--remove-orphans", "--timeout", "5")
	}
	if resourcesRemain(cleanupContext, project) {
		if len(environment) > 0 {
			_, _ = composeCommand(cleanupContext, layout, project, environment, "down", "--volumes", "--remove-orphans", "--timeout", "5")
		}
		if resourcesRemain(cleanupContext, project) {
			cleanupErr = errors.Join(cleanupErr, errors.New("compose project resources remain after repeated cleanup"))
		}
	}
	if cleanupErr == nil {
		summary.Checks["cleanup_complete"] = true
	}
	if runErr != nil {
		summary.Status = "failed"
		summary.Failure = runErr.Error()
	}
	if err := removeSmokeRunDirectory(layout); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
		summary.Checks["cleanup_complete"] = false
	}
	if cleanupErr != nil {
		summary.Status = "failed"
		summary.Failure = errors.Join(runErr, cleanupErr).Error()
	}
	evidenceErr := writeBoundedJSON(filepath.Join(layout.evidenceDir, "compose-smoke.json"), summary)
	return errors.Join(runErr, cleanupErr, evidenceErr)
}

func resourcesRemain(ctx context.Context, project string) bool {
	queries := [][]string{
		{"ps", "--all", "--quiet", "--filter", "label=com.docker.compose.project=" + project},
		{"network", "ls", "--quiet", "--filter", "label=com.docker.compose.project=" + project},
		{"volume", "ls", "--quiet", "--filter", "label=com.docker.compose.project=" + project},
	}
	for _, arguments := range queries {
		output, err := exec.CommandContext(ctx, "docker", arguments...).Output()
		if err != nil || strings.TrimSpace(string(output)) != "" {
			return true
		}
	}
	return false
}

func composeProjectName(runID string) string {
	digest := sha256.Sum256([]byte(runID))
	return "ardents-lab-" + hex.EncodeToString(digest[:8])
}

func validImageID(image string) bool {
	algorithm, value, found := strings.Cut(image, ":")
	digest, err := hex.DecodeString(value)
	return found && algorithm == "sha256" && err == nil && len(digest) == sha256.Size
}

func rolesOnlySawAllowedPeer(roles map[string]smokeRoleResult) bool {
	for role, result := range roles {
		peer := "alpha"
		if role == "alpha" {
			peer = "beta"
		}
		if len(result.ObservedPeers) != 2 || result.ObservedPeers[0] != peer || result.ObservedPeers[1] != peer {
			return false
		}
	}
	return len(roles) == 2
}

func containsNoNewPrivileges(values []string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, "no-new-privileges") {
			return true
		}
	}
	return false
}

func isolatedMounts(mounts []struct {
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}, _ string) bool {
	if len(mounts) != 2 {
		return false
	}
	seenConfig, seenEvidence := false, false
	for _, mount := range mounts {
		switch mount.Destination {
		case "/config/role.json":
			seenConfig = !mount.RW
		case "/evidence":
			seenEvidence = mount.RW
		}
	}
	return seenConfig && seenEvidence
}

func allChecksPassed(checks map[string]bool, ignored string) bool {
	for name, passed := range checks {
		if name != ignored && !passed {
			return false
		}
	}
	return true
}

func officialUbuntuRunner() bool {
	kernelRelease, kernelErr := os.ReadFile("/proc/sys/kernel/osrelease")
	osRelease, releaseErr := os.ReadFile("/etc/os-release")
	return kernelErr == nil && releaseErr == nil && officialRunnerFor(runtime.GOOS, runtime.GOARCH, string(kernelRelease), osRelease)
}

func officialRunnerFor(goos, goarch, kernelRelease string, osRelease []byte) bool {
	if goos != "linux" || goarch != "amd64" || strings.Contains(strings.ToLower(kernelRelease), "microsoft") {
		return false
	}
	values := make(map[string]string)
	for line := range strings.SplitSeq(string(osRelease), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || strings.HasPrefix(key, "#") {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), "\"'")
	}
	return values["ID"] == "ubuntu" && values["VERSION_ID"] == "26.04"
}

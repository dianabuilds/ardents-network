package tooling

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type composeNetworkInspect struct {
	Name       string                     `json:"Name"`
	Internal   bool                       `json:"Internal"`
	Containers map[string]json.RawMessage `json:"Containers"`
}

type composeContainerInspect struct {
	Config struct {
		User string `json:"User"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode    string   `json:"NetworkMode"`
		ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
		Privileged     bool     `json:"Privileged"`
		CapAdd         []string `json:"CapAdd"`
		CapDrop        []string `json:"CapDrop"`
		SecurityOpt    []string `json:"SecurityOpt"`
		PidsLimit      int64    `json:"PidsLimit"`
		Memory         int64    `json:"Memory"`
		NanoCPUs       int64    `json:"NanoCpus"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports    map[string]json.RawMessage `json:"Ports"`
		Networks map[string]json.RawMessage `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
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

func containsNoNewPrivileges(values []string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, "no-new-privileges") {
			return true
		}
	}
	return false
}

func smokeNetworkContract(networks []composeNetworkInspect, expectedName string) bool {
	return len(networks) == 1 && networks[0].Name == expectedName && networks[0].Internal
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
	if kernelErr != nil || releaseErr != nil || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || strings.Contains(strings.ToLower(string(kernelRelease)), "microsoft") {
		return false
	}
	for line := range strings.SplitSeq(string(osRelease), "\n") {
		if strings.TrimSpace(line) == "ID=ubuntu" {
			return strings.Contains(string(osRelease), "VERSION_ID=\"26.04\"")
		}
	}
	return false
}

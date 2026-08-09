package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

const composeSmokeSchema = "carrier-lab-compose-smoke/v1"

type runLayout struct {
	identity       preflight.RunLayout
	runID          string
	repositoryRoot string
	runDir         string
	evidenceDir    string
}

type composeSmokeSummary struct {
	SchemaVersion  string                     `json:"schema_version"`
	RunID          string                     `json:"run_id"`
	Project        string                     `json:"compose_project"`
	Classification string                     `json:"classification"`
	Status         string                     `json:"status"`
	Checks         map[string]bool            `json:"checks"`
	Roles          map[string]smokeRoleResult `json:"roles,omitempty"`
	Failure        string                     `json:"failure,omitempty"`
}

type composeContainerInspect struct {
	Config struct {
		User string `json:"User"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode    string   `json:"NetworkMode"`
		ReadonlyRootfs bool     `json:"ReadonlyRootfs"`
		Privileged     bool     `json:"Privileged"`
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
	State composeContainerState `json:"State"`
}

type composeContainerState struct {
	Running  bool `json:"Running"`
	ExitCode int  `json:"ExitCode"`
}

type composeNetworkInspect struct {
	Name     string `json:"Name"`
	Internal bool   `json:"Internal"`
}

// Run owns one fixed two-process Compose lifecycle. A fault value
// of "controller-stop" interrupts after readiness to exercise cleanup.
func Run(ctx context.Context, identity preflight.RunLayout, image, fault string) (evidenceDir string, runErr error) {
	layout, err := ownedLayout(identity, false, false)
	if err != nil {
		return "", err
	}
	if !validImageID(image) {
		return "", errors.New("image must be an immutable sha256 Carrier Lab image ID")
	}
	if fault != "" && fault != "controller-stop" {
		return "", errors.New("unsupported isolation smoke fault")
	}
	if err := prepareSmokeWorkspace(layout); err != nil {
		return "", err
	}
	project := composeProjectName(layout.runID)
	summary := composeSmokeSummary{
		SchemaVersion: composeSmokeSchema,
		RunID:         layout.runID,
		Project:       project,
		Classification: func() string {
			if officialUbuntuRunner() {
				return "official"
			}
			return "development"
		}(),
		Status: "failed",
		Checks: map[string]bool{
			"allowed_peer_only": false, "single_internal_network": false,
			"no_host_ports": false, "security_controls": false, "cleanup_complete": false,
		},
	}
	evidenceDir = layout.evidenceDir
	environment, err := prepareSmokeInputs(layout, image)
	if err != nil {
		return evidenceDir, finishSmoke(layout, environment, &summary, err)
	}
	defer func() {
		runErr = finishSmoke(layout, environment, &summary, runErr)
	}()

	if _, err := composeCommand(ctx, layout, project, environment, "up", "--detach", "--no-build", "--pull", "never"); err != nil {
		return evidenceDir, fmt.Errorf("compose up: %w", err)
	}
	if err := waitForSmokeReadiness(ctx, layout, 15*time.Second); err != nil {
		return evidenceDir, err
	}
	if fault == "controller-stop" {
		return evidenceDir, errors.New("injected controller stop after readiness")
	}
	checks, containerIDs, err := inspectSmokeContainers(ctx, layout, project, environment)
	if err != nil {
		return evidenceDir, err
	}
	for name, passed := range checks {
		summary.Checks[name] = passed
	}
	if err := waitForSmokeContainers(ctx, containerIDs, 15*time.Second); err != nil {
		return evidenceDir, fmt.Errorf("wait for smoke roles: %w", err)
	}
	roles, err := readSmokeResults(layout)
	if err != nil {
		return evidenceDir, err
	}
	summary.Roles = roles
	summary.Checks["allowed_peer_only"] = rolesOnlySawAllowedPeer(roles)
	for _, role := range roles {
		if role.Status != "passed" {
			return evidenceDir, errors.New("one or more smoke roles failed")
		}
	}
	if !allChecksPassed(summary.Checks, "cleanup_complete") {
		return evidenceDir, errors.New("compose isolation inspection failed")
	}
	summary.Status = "passed"
	return evidenceDir, nil
}

func ownedLayout(identity preflight.RunLayout, requireRun, requireEvidence bool) (runLayout, error) {
	runID, repositoryRoot, runDir, evidenceDir, err := identity.OwnedPaths(requireRun, requireEvidence)
	if err != nil {
		return runLayout{}, err
	}
	return runLayout{identity: identity, runID: runID, repositoryRoot: repositoryRoot, runDir: runDir, evidenceDir: evidenceDir}, nil
}

func prepareSmokeWorkspace(layout runLayout) error {
	if _, err := ownedLayout(layout.identity, false, false); err != nil {
		return err
	}
	for _, directory := range []string{layout.runDir, layout.evidenceDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return fmt.Errorf("create smoke workspace: %w", err)
		}
	}
	_, err := ownedLayout(layout.identity, true, true)
	return err
}

func prepareSmokeInputs(layout runLayout, image string) ([]string, error) {
	configs := filepath.Join(layout.runDir, "configs")
	alphaEvidence := filepath.Join(layout.runDir, "evidence-alpha")
	betaEvidence := filepath.Join(layout.runDir, "evidence-beta")
	for _, directory := range []string{configs, alphaEvidence, betaEvidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return nil, err
		}
	}
	for _, directory := range []string{alphaEvidence, betaEvidence} {
		if err := os.Chmod(directory, 0o777); err != nil {
			return nil, err
		}
	}
	for _, config := range []smokeRoleConfig{
		{SchemaVersion: smokeRoleSchema, RunID: layout.runID, Role: "alpha", ListenAddress: ":37001", PeerRole: "beta", PeerAddress: "beta:37001"},
		{SchemaVersion: smokeRoleSchema, RunID: layout.runID, Role: "beta", ListenAddress: ":37001", PeerRole: "alpha", PeerAddress: "alpha:37001"},
	} {
		if err := writeBoundedJSON(filepath.Join(configs, config.Role+".json"), config); err != nil {
			return nil, err
		}
	}
	return append(os.Environ(),
		"CARRIER_LAB_IMAGE="+image,
		"ALPHA_CONFIG="+filepath.Join(configs, "alpha.json"),
		"BETA_CONFIG="+filepath.Join(configs, "beta.json"),
		"ALPHA_EVIDENCE="+alphaEvidence,
		"BETA_EVIDENCE="+betaEvidence,
	), nil
}

func composeCommand(ctx context.Context, layout runLayout, project string, environment []string, arguments ...string) ([]byte, error) {
	base := []string{"compose", "--project-name", project, "--file", filepath.Join(layout.repositoryRoot, "compose.carrier-lab.yaml")}
	command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func waitForSmokeReadiness(ctx context.Context, layout runLayout, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ready := true
		for _, role := range []string{"alpha", "beta"} {
			path := filepath.Join(layout.runDir, "evidence-"+role, "ready.json")
			if info, err := os.Stat(path); err != nil || info.Size() > smokeEvidenceCap {
				ready = false
			}
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("smoke roles did not become ready before the deadline")
}

func inspectSmokeContainers(ctx context.Context, layout runLayout, project string, environment []string) (map[string]bool, map[string]string, error) {
	checks := map[string]bool{"single_internal_network": true, "no_host_ports": true, "security_controls": true}
	containerIDs := make(map[string]string, 2)
	expectedNetwork := project + "_adjacency"
	for _, role := range []string{"alpha", "beta"} {
		containerID, err := composeCommand(ctx, layout, project, environment, "ps", "--all", "--quiet", role)
		if err != nil {
			return nil, nil, err
		}
		containerIDs[role] = strings.TrimSpace(string(containerID))
		if containerIDs[role] == "" {
			return nil, nil, fmt.Errorf("compose did not retain the %s smoke container", role)
		}
		command := exec.CommandContext(ctx, "docker", "inspect", containerIDs[role])
		data, err := command.Output()
		if err != nil {
			return nil, nil, err
		}
		var inspected []composeContainerInspect
		if err := json.Unmarshal(data, &inspected); err != nil || len(inspected) != 1 {
			return nil, nil, errors.New("cannot inspect smoke container")
		}
		container := inspected[0]
		checks["single_internal_network"] = checks["single_internal_network"] && len(container.NetworkSettings.Networks) == 1 && container.NetworkSettings.Networks[expectedNetwork] != nil
		checks["no_host_ports"] = checks["no_host_ports"] && len(container.NetworkSettings.Ports) == 0
		checks["security_controls"] = checks["security_controls"] && container.Config.User == "65532:65532" &&
			container.HostConfig.ReadonlyRootfs && !container.HostConfig.Privileged && container.HostConfig.NetworkMode != "host" &&
			len(container.HostConfig.CapDrop) == 1 && container.HostConfig.CapDrop[0] == "ALL" &&
			containsNoNewPrivileges(container.HostConfig.SecurityOpt) && container.HostConfig.PidsLimit > 0 &&
			container.HostConfig.Memory > 0 && container.HostConfig.NanoCPUs > 0 && isolatedMounts(container.Mounts, role)
	}
	networkData, err := exec.CommandContext(ctx, "docker", "network", "inspect", expectedNetwork).Output()
	if err != nil {
		return nil, nil, fmt.Errorf("inspect smoke network: %w", err)
	}
	var networks []composeNetworkInspect
	if err := json.Unmarshal(networkData, &networks); err != nil {
		return nil, nil, fmt.Errorf("decode smoke network inspection: %w", err)
	}
	checks["single_internal_network"] = checks["single_internal_network"] && smokeNetworkContract(networks, expectedNetwork)
	return checks, containerIDs, nil
}

func smokeNetworkContract(networks []composeNetworkInspect, expectedName string) bool {
	return len(networks) == 1 && networks[0].Name == expectedName && networks[0].Internal
}

func waitForSmokeContainers(ctx context.Context, containerIDs map[string]string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allCompleted := true
		for _, role := range []string{"alpha", "beta"} {
			command := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .State}}", containerIDs[role])
			data, err := command.Output()
			if err != nil {
				return fmt.Errorf("inspect %s smoke container: %w", role, err)
			}
			var state composeContainerState
			if err := json.Unmarshal(data, &state); err != nil {
				return fmt.Errorf("decode %s smoke container state: %w", role, err)
			}
			completed, err := smokeContainerOutcome(state)
			if err != nil {
				return fmt.Errorf("%s smoke container: %w", role, err)
			}
			allCompleted = allCompleted && completed
		}
		if allCompleted {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("smoke containers did not exit before the deadline")
}

func smokeContainerOutcome(state composeContainerState) (bool, error) {
	if state.Running {
		return false, nil
	}
	if state.ExitCode != 0 {
		return true, fmt.Errorf("exited with status %d", state.ExitCode)
	}
	return true, nil
}

func readSmokeResults(layout runLayout) (map[string]smokeRoleResult, error) {
	roles := make(map[string]smokeRoleResult, 2)
	for _, role := range []string{"alpha", "beta"} {
		path := filepath.Join(layout.runDir, "evidence-"+role, "result.json")
		info, err := os.Stat(path)
		if err != nil || info.Size() > smokeEvidenceCap {
			return nil, errors.New("missing or oversized role evidence")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var result smokeRoleResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		roles[role] = result
	}
	return roles, nil
}

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

func removeSmokeRunDirectory(layout runLayout) error {
	if _, err := ownedLayout(layout.identity, false, true); err != nil {
		return err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return err
	}
	if _, err := os.Stat(layout.runDir); !os.IsNotExist(err) {
		return errors.New("smoke run directory remains after cleanup")
	}
	return nil
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

package carrier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
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
		"CARRIER_LAB_RUN="+layout.runDir,
		"TOOLING_RUN_ID="+layout.runID,
	), nil
}

func composeCommand(ctx context.Context, layout runLayout, project string, environment []string, arguments ...string) ([]byte, error) {
	base := []string{"compose", "--project-name", project, "--file", carrierLabComposePath(layout.repositoryRoot), "--profile", "isolation"}
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

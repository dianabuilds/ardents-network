package tooling

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

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

const (
	toolingSmokeSchema   = "carrier-lab-tooling-manifest/v2"
	toolingVerdictSchema = "carrier-lab-tooling-verdict/v1"
	toolingManifestFile  = "tooling-manifest.json"
	toolingVerdictFile   = "tooling-verdict.json"
)

var toolingRoles = []string{"tracer-alpha", "tracer-beta", "shape-alpha", "shape-beta", "capture-alpha"}

type toolingSmokeSummary struct {
	SchemaVersion         string                       `json:"schema_version"`
	RunID                 string                       `json:"run_id"`
	Classification        string                       `json:"classification"`
	Status                string                       `json:"status"`
	ToolingImageID        string                       `json:"tooling_image_id"`
	ToolLockSHA256        string                       `json:"tool_lock_sha256,omitempty"`
	BuildReceipt          *toolingBuildReceipt         `json:"build_receipt,omitempty"`
	Checks                map[string]bool              `json:"checks"`
	EffectiveCapabilities map[string]string            `json:"effective_capabilities,omitempty"`
	Tools                 map[string]observedTool      `json:"tools,omitempty"`
	QdiscState            map[string]string            `json:"qdisc_state,omitempty"`
	CaptureSHA256         string                       `json:"capture_sha256,omitempty"`
	CaptureBytes          int64                        `json:"capture_bytes,omitempty"`
	Roles                 map[string]toolingRoleResult `json:"roles,omitempty"`
	Failure               string                       `json:"failure,omitempty"`
}

type toolingSmokeVerdict struct {
	SchemaVersion  string `json:"schema_version"`
	RunID          string `json:"run_id"`
	Status         string `json:"status"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

// RunSmoke owns one real offline tc/tcpdump Compose lifecycle. A fault
// of "capture-start" or "shaping" exercises fail-closed cleanup.
func RunSmoke(ctx context.Context, identity preflight.RunLayout, image, fault string) (evidenceDirectory string, runErr error) {
	layout, err := ownedLayout(identity, false, false)
	if err != nil {
		return "", err
	}
	if !validImageID(image) {
		return "", errors.New("tooling image must be an immutable sha256 image ID")
	}
	if fault != "" && fault != "capture-start" && fault != "shaping" {
		return "", errors.New("unsupported tooling smoke fault")
	}
	summary := toolingSmokeSummary{
		SchemaVersion: toolingSmokeSchema, RunID: layout.runID, Status: "failed", ToolingImageID: image,
		Classification: func() string {
			if officialUbuntuRunner() {
				return "official"
			}
			return "development"
		}(),
		Checks: map[string]bool{}, EffectiveCapabilities: map[string]string{}, QdiscState: map[string]string{},
	}
	for _, check := range requiredToolingChecks {
		summary.Checks[check] = false
	}
	evidenceDirectory = layout.evidenceDir
	project := composeProjectName("tooling-" + layout.runID)
	capturePath := filepath.Join(layout.runDir, "raw-capture", "alpha-link.pcap")
	var environment []string
	defer func() {
		runErr = finishToolingSmoke(layout, project, environment, capturePath, &summary, runErr)
	}()
	if err := prepareSmokeWorkspace(layout); err != nil {
		return evidenceDirectory, err
	}
	receipt, err := inspectToolingBuildReceipt(ctx, layout, project, image)
	if err != nil {
		return evidenceDirectory, err
	}
	summary.BuildReceipt = &receipt
	summary.ToolLockSHA256 = receipt.ToolLockSHA256
	summary.Checks["image_receipt"] = true
	environment, err = prepareToolingInputs(layout, image, fault, capturePath)
	if err != nil {
		return evidenceDirectory, err
	}
	if _, err := toolingComposeCommand(ctx, layout, project, environment, "up", "--detach", "--no-build", "--pull", "never"); err != nil {
		return evidenceDirectory, fmt.Errorf("tooling compose up: %w", err)
	}
	if err := waitForToolingReadiness(ctx, layout, 10*time.Second); err != nil {
		return evidenceDirectory, err
	}
	isolation, err := inspectToolingIsolation(ctx, layout, project, environment)
	if err != nil {
		return evidenceDirectory, err
	}
	summary.Checks["isolation"] = isolation
	if err := writeControlFile(layout, "start"); err != nil {
		return evidenceDirectory, err
	}
	if err := waitForToolingExchange(ctx, layout, 12*time.Second); err != nil {
		return evidenceDirectory, err
	}
	if err := waitForToolingResults(ctx, layout, []string{"capture-alpha"}, 12*time.Second); err != nil {
		return evidenceDirectory, err
	}
	if err := writeControlFile(layout, "stop"); err != nil {
		return evidenceDirectory, err
	}
	if err := waitForToolingResults(ctx, layout, toolingRoles, 12*time.Second); err != nil {
		return evidenceDirectory, err
	}
	roles, err := readToolingResults(layout)
	if err != nil {
		return evidenceDirectory, err
	}
	summary.Roles = roles
	if err := evaluateToolingResults(layout, roles, &summary); err != nil {
		return evidenceDirectory, err
	}
	if !allChecksPassed(summary.Checks, "cleanup_complete") {
		return evidenceDirectory, errors.New("tooling observations failed before cleanup")
	}
	return evidenceDirectory, nil
}

func prepareToolingInputs(layout runLayout, image, fault, capturePath string) ([]string, error) {
	directories := []string{"control", "tracer-alpha", "tracer-beta", "shape-alpha", "shape-beta", "capture-alpha", "raw-capture"}
	for _, name := range directories {
		path := filepath.Join(layout.runDir, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			return nil, err
		}
		if err := os.Chmod(path, 0o777); err != nil {
			return nil, err
		}
	}
	if err := validateRawCapturePath(capturePath, layout.runDir, layout.repositoryRoot); err != nil {
		return nil, err
	}
	shapingFault, captureFault := "", ""
	if fault == "shaping" {
		shapingFault = fault
	}
	if fault == "capture-start" {
		captureFault = fault
	}
	environment := append(os.Environ(),
		"CARRIER_LAB_IMAGE="+image,
		"CARRIER_LAB_RUN="+layout.runDir,
		"TOOLING_RUN_ID="+layout.runID,
		"SHAPING_ALPHA_FAULT="+shapingFault,
		"CAPTURE_FAULT="+captureFault,
	)
	return environment, nil
}

func toolingComposeCommand(ctx context.Context, layout runLayout, project string, environment []string, arguments ...string) ([]byte, error) {
	base := []string{"compose", "--project-name", project, "--file", carrierLabComposePath(layout.repositoryRoot), "--profile", "tooling"}
	command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func waitForToolingReadiness(ctx context.Context, layout runLayout, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ready := true
		for _, role := range toolingRoles {
			if _, err := os.Stat(filepath.Join(layout.runDir, role, "ready.json")); err != nil {
				ready = false
				resultPath := filepath.Join(layout.runDir, role, "result.json")
				if data, resultErr := os.ReadFile(resultPath); resultErr == nil && len(data) <= smokeEvidenceCap {
					var result toolingRoleResult
					if json.Unmarshal(data, &result) == nil {
						return fmt.Errorf("%s failed before readiness: %s", role, result.Failure)
					}
					return fmt.Errorf("%s published invalid failure evidence", role)
				}
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
	return errors.New("tooling roles did not become ready before the deadline")
}

func waitForToolingResults(ctx context.Context, layout runLayout, roles []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		complete := true
		for _, role := range roles {
			if _, err := os.Stat(filepath.Join(layout.runDir, role, "result.json")); err != nil {
				complete = false
			}
		}
		if complete {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("tooling roles did not publish results before the deadline")
}

func waitForToolingExchange(ctx context.Context, layout runLayout, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		complete := true
		for _, role := range []string{"tracer-alpha", "tracer-beta"} {
			if _, err := os.Stat(filepath.Join(layout.runDir, role, "exchange.json")); err != nil {
				complete = false
			}
		}
		if complete {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("synthetic tracer exchange did not complete before the deadline")
}

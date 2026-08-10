package nativecircuit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/harness/tooling"
	"github.com/dianabuilds/ardents-network/internal/preflight"
)

const nativeRunSchema = "carrier-lab-native-run/v1"

type nativeRunLayout struct {
	identity       preflight.RunLayout
	runID          string
	repositoryRoot string
	runDirectory   string
	evidenceDir    string
}

type nativeRunSummary struct {
	SchemaVersion    string                      `json:"schema_version"`
	RunID            string                      `json:"run_id"`
	Profile          string                      `json:"profile"`
	Classification   string                      `json:"classification"`
	Status           string                      `json:"status"`
	Verdict          string                      `json:"verdict"`
	Checks           map[string]bool             `json:"checks"`
	ApplicationImage string                      `json:"application_image"`
	ToolImage        string                      `json:"tool_image"`
	ImageReceipt     *tooling.NativeImageReceipt `json:"image_receipt,omitempty"`
	Fault            string                      `json:"fault,omitempty"`
	Failure          string                      `json:"failure,omitempty"`
}

// Run owns one native C-5/C2 development smoke lifecycle. It proves the fixed
// topology, protected stream, real per-link shaping/capture, evidence, and
// cleanup; it does not claim the full R-013 sampling verdict.
func Run(ctx context.Context, identity preflight.RunLayout, applicationImage, toolImage, fault string) (evidenceDir string, runErr error) {
	layout, err := openNativeRunLayout(identity, false, false)
	if err != nil {
		return "", err
	}
	if !validNativeImageID(applicationImage) || !validNativeImageID(toolImage) {
		return "", errors.New("native run images must be immutable sha256 image IDs")
	}
	if fault != "" && fault != "rendezvous-process" {
		return "", errors.New("native run fault is not part of the fixed contract")
	}
	for _, directory := range []string{layout.runDirectory, layout.evidenceDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return "", err
		}
	}
	if _, err := openNativeRunLayout(identity, true, true); err != nil {
		return "", err
	}
	fixture, err := prepareNativeFixture(layout.runDirectory, layout.runID, fault)
	if err != nil {
		return layout.evidenceDir, err
	}
	project := nativeProjectName(layout.runID)
	environment := nativeEnvironment(fixture, applicationImage, toolImage)
	summary := nativeRunSummary{
		SchemaVersion: nativeRunSchema, RunID: layout.runID, Profile: candidateProfile,
		Classification: nativeRunnerClassification(), Status: "failed", Verdict: "invalid",
		ApplicationImage: applicationImage, ToolImage: toolImage, Fault: fault,
		Checks: nativeRunChecks(fault),
	}
	evidenceDir = layout.evidenceDir
	defer func() { runErr = finishNativeRun(layout, fixture, project, environment, &summary, runErr) }()
	receipt, err := tooling.VerifyNativeImages(ctx, layout.repositoryRoot, project, applicationImage, toolImage)
	if err != nil {
		return evidenceDir, err
	}
	summary.ImageReceipt = &receipt
	summary.Checks["verified_images"] = true
	if _, err := nativeCompose(ctx, layout, project, environment, "up", "--detach", "--no-build", "--pull", "never"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeReady(ctx, fixture, initialNativeReadyPaths(fixture), 30*time.Second); err != nil {
		return evidenceDir, err
	}
	inspection, err := inspectNativeProject(ctx, layout, project, environment)
	if err != nil {
		return evidenceDir, err
	}
	summary.Checks["fixed_topology"] = inspection.FixedTopology
	summary.Checks["bounded_capabilities"] = inspection.BoundedCapabilities
	if !inspection.FixedTopology || !inspection.BoundedCapabilities {
		return evidenceDir, errors.New("native Compose topology or capability inspection failed")
	}
	if err := writeControlMarker(fixture.controlDirectory, "nodes-ready"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeReady(ctx, fixture, []string{filepath.Join(fixture.roleEvidence["service"], "ready.json")}, 15*time.Second); err != nil {
		return evidenceDir, err
	}
	if err := writeControlMarker(fixture.controlDirectory, "user-start"); err != nil {
		return evidenceDir, err
	}
	if fault == "rendezvous-process" {
		return evidenceDir, runNativeRendezvousFailure(ctx, layout, fixture, project, environment, &summary)
	}
	if err := waitNativeReady(ctx, fixture, []string{
		filepath.Join(fixture.roleEvidence["user"], "attempt-ready.json"),
		filepath.Join(fixture.roleEvidence["service"], "attempt-ready.json"),
	}, 30*time.Second); err != nil {
		return evidenceDir, err
	}
	if err := writeControlMarker(fixture.controlDirectory, "stop"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeReady(ctx, fixture, nativeCaptureReadyPaths(fixture), 30*time.Second); err != nil {
		return evidenceDir, err
	}
	if err := inspectForbiddenSentinels(fixture.captureDirectory, fixture.forbiddenSentinels); err != nil {
		return evidenceDir, err
	}
	summary.Checks["forbidden_sentinels_absent"] = true
	if err := writeControlMarker(fixture.controlDirectory, "capture-cleanup"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeServices(ctx, layout, project, environment, nativeComposeServices(), 30*time.Second); err != nil {
		return evidenceDir, err
	}
	if err := collectNativeEvidence(fixture, layout.evidenceDir, &summary); err != nil {
		return evidenceDir, err
	}
	summary.Status = "passed"
	summary.Verdict = "development_smoke_passed"
	return evidenceDir, nil
}

func openNativeRunLayout(identity preflight.RunLayout, requireRun, requireEvidence bool) (nativeRunLayout, error) {
	runID, repositoryRoot, runDirectory, evidenceDir, err := identity.OwnedPaths(requireRun, requireEvidence)
	if err != nil {
		return nativeRunLayout{}, err
	}
	return nativeRunLayout{identity: identity, runID: runID, repositoryRoot: repositoryRoot, runDirectory: runDirectory, evidenceDir: evidenceDir}, nil
}

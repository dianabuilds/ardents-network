package nativecircuit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/lab/preflight"
	"github.com/dianabuilds/ardents-network/internal/lab/tooling"
)

const nativeRunSchema = "carrier-lab-native-run/v1"

var errNativeContractFailure = errors.New("native candidate contract failure")

func nativeContractFailure(err error) error {
	if err == nil {
		return nil
	}
	return errors.Join(errNativeContractFailure, err)
}

func nativeFailureKind(runErr, cleanupErr error) string {
	if cleanupErr == nil && errors.Is(runErr, errNativeContractFailure) {
		return "scenario"
	}
	return "operational"
}

type nativeRunLayout struct {
	identity       ownedRunLayout
	runID          string
	repositoryRoot string
	runDirectory   string
	evidenceDir    string
	shared         bool
}

type ownedRunLayout interface {
	OwnedPaths(requireRun, requireEvidence bool) (string, string, string, string, error)
}

type nativeRunSummary struct {
	SchemaVersion       string                      `json:"schema_version"`
	RunID               string                      `json:"run_id"`
	Profile             string                      `json:"profile"`
	Classification      string                      `json:"classification"`
	Status              string                      `json:"status"`
	Verdict             string                      `json:"verdict"`
	Checks              map[string]bool             `json:"checks"`
	ApplicationImage    string                      `json:"application_image"`
	ToolImage           string                      `json:"tool_image"`
	ImageReceipt        *tooling.NativeImageReceipt `json:"image_receipt,omitempty"`
	Fault               string                      `json:"fault,omitempty"`
	Failure             string                      `json:"failure,omitempty"`
	FailureKind         string                      `json:"failure_kind,omitempty"`
	Workload            *nativeWorkload             `json:"workload,omitempty"`
	SetupMilliseconds   int64                       `json:"setup_milliseconds"`
	FailureMilliseconds int64                       `json:"failure_milliseconds,omitempty"`
}

// Run owns one native C-5/C2 development smoke lifecycle. It proves the fixed
// topology, protected stream, real per-link shaping/capture, evidence, and
// cleanup; it does not claim the full R-013 sampling verdict.
func Run(ctx context.Context, identity preflight.RunLayout, applicationImage, toolImage, fault string) (evidenceDir string, runErr error) {
	return runNative(ctx, identity, applicationImage, toolImage, fault, nil, nil)
}

// RunWorkload owns one exact setup or 60-second stream attempt for the frozen
// R-013 comparative experiment. The immutable images are supplied by ID and
// never built by this function.
func RunWorkload(ctx context.Context, identity preflight.RunLayout, applicationImage, toolImage, workloadPath string) (evidenceDir string, runErr error) {
	workload, err := readNativeWorkload(workloadPath)
	if err != nil {
		return "", err
	}
	return runNative(ctx, identity, applicationImage, toolImage, "", &workload, nil)
}

func runNative(ctx context.Context, identity ownedRunLayout, applicationImage, toolImage, fault string, workload *nativeWorkload, attached *attachedSpec) (evidenceDir string, runErr error) {
	topology := topologyFor(workload)
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
	layout.shared = attached != nil
	for _, directory := range []string{layout.runDirectory, layout.evidenceDir} {
		if err := ensureNativeDirectory(directory, attached != nil); err != nil {
			return "", err
		}
	}
	if _, err := openNativeRunLayout(identity, true, true); err != nil {
		return "", err
	}
	fixture, err := prepareNativeFixtureMode(layout.runDirectory, layout.runID, fault, workload, attached)
	if err != nil {
		return layout.evidenceDir, err
	}
	project := nativeProjectName(layout.runID)
	environment := nativeEnvironment(fixture, applicationImage, toolImage, attached)
	roleProfile := candidateProfile
	if topology.profile == "c3" {
		roleProfile = c3Profile
	} else if topology.profile == "direct" {
		roleProfile = directProfile
	}
	summary := nativeRunSummary{
		SchemaVersion: nativeRunSchema, RunID: layout.runID, Profile: roleProfile,
		Classification: nativeRunnerClassification(), Status: "failed", Verdict: "invalid",
		ApplicationImage: applicationImage, ToolImage: toolImage, Fault: fault,
		Checks:   nativeRunChecks(fault),
		Workload: workload,
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
	if err := waitNativeReady(ctx, fixture, initialNativeReadyPaths(fixture, topology), 30*time.Second); err != nil {
		return evidenceDir, err
	}
	inspection, err := inspectNativeProject(ctx, layout, project, environment, topology)
	if err != nil {
		return evidenceDir, err
	}
	summary.Checks["fixed_topology"] = inspection.FixedTopology
	summary.Checks["bounded_capabilities"] = inspection.BoundedCapabilities
	if !inspection.FixedTopology || !inspection.BoundedCapabilities {
		return evidenceDir, nativeContractFailure(errors.New("native Compose topology or capability inspection failed"))
	}
	if err := writeControlMarker(fixture.controlDirectory, "nodes-ready"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeReady(ctx, fixture, []string{filepath.Join(fixture.roleEvidence["service"], "ready.json")}, 15*time.Second); err != nil {
		return evidenceDir, err
	}
	var sampler *resourceSampler
	if workload != nil && workload.Kind == "stream" || attached != nil {
		states, err := inspectNativeServiceStates(ctx, layout, project, environment)
		if err != nil {
			return evidenceDir, err
		}
		roles := make(map[string]string, len(topology.applicationRoles))
		for _, role := range topology.applicationRoles {
			container, found := states[role]
			if !found {
				return evidenceDir, errors.New("native resource sampler process set is incomplete")
			}
			roles[container.ID] = role
		}
		initialSamples, err := readDockerResources(ctx, roles, 0)
		if err != nil || len(initialSamples) != len(roles) {
			return evidenceDir, errors.Join(err, errors.New("initial native resource sample is incomplete"))
		}
		sampler = startResourceSampler(ctx, roles, initialSamples)
		defer sampler.stop()
	}
	if err := writeControlMarker(fixture.controlDirectory, "user-start"); err != nil {
		return evidenceDir, err
	}
	setupStarted := time.Now()
	if err := waitNativeReady(ctx, fixture, []string{filepath.Join(fixture.roleEvidence["user"], "setup-ready.json")}, 30*time.Second); err != nil {
		return evidenceDir, err
	}
	summary.SetupMilliseconds = time.Since(setupStarted).Milliseconds()
	if fault == "rendezvous-process" {
		return evidenceDir, runNativeRendezvousFailure(ctx, layout, fixture, project, environment, &summary)
	}
	attemptTimeout := 30 * time.Second
	if workload != nil && workload.Kind == "stream" || attached != nil {
		attemptTimeout = 90 * time.Second
	}
	if err := waitNativeReady(ctx, fixture, []string{
		filepath.Join(fixture.roleEvidence["user"], "attempt-ready.json"),
		filepath.Join(fixture.roleEvidence["service"], "attempt-ready.json"),
	}, attemptTimeout); err != nil {
		if attached != nil {
			scenario, evidenceErr := attachedRoleScenarioFailure(fixture)
			if evidenceErr != nil {
				return evidenceDir, errors.Join(err, evidenceErr)
			}
			if scenario {
				return evidenceDir, nativeContractFailure(err)
			}
		}
		return evidenceDir, err
	}
	if sampler != nil {
		samples, err := sampler.stop()
		if err != nil || len(samples) == 0 {
			return evidenceDir, errors.Join(err, errors.New("native resource samples are incomplete"))
		}
		data, err := marshalResourceSamples(samples)
		if err != nil {
			return evidenceDir, err
		}
		if err := os.WriteFile(filepath.Join(layout.evidenceDir, "resource-samples.json"), append(data, '\n'), 0o600); err != nil {
			return evidenceDir, err
		}
	}
	if err := writeControlMarker(fixture.controlDirectory, "stop"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeReady(ctx, fixture, nativeCaptureReadyPaths(fixture), 30*time.Second); err != nil {
		return evidenceDir, err
	}
	if err := inspectForbiddenSentinels(fixture.captureDirectory, fixture.forbiddenSentinels, len(topology.networkRoles)); err != nil {
		return evidenceDir, err
	}
	summary.Checks["forbidden_sentinels_absent"] = true
	if err := writeControlMarker(fixture.controlDirectory, "capture-cleanup"); err != nil {
		return evidenceDir, err
	}
	if err := waitNativeServices(ctx, layout, project, environment, topology.services(), 30*time.Second); err != nil {
		return evidenceDir, err
	}
	if err := collectNativeEvidence(fixture, layout.evidenceDir, &summary); err != nil {
		return evidenceDir, err
	}
	summary.Status = "passed"
	summary.Verdict = "development_smoke_passed"
	return evidenceDir, nil
}

func openNativeRunLayout(identity ownedRunLayout, requireRun, requireEvidence bool) (nativeRunLayout, error) {
	runID, repositoryRoot, runDirectory, evidenceDir, err := identity.OwnedPaths(requireRun, requireEvidence)
	if err != nil {
		return nativeRunLayout{}, err
	}
	return nativeRunLayout{identity: identity, runID: runID, repositoryRoot: repositoryRoot, runDirectory: runDirectory, evidenceDir: evidenceDir}, nil
}

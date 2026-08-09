package preflight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const bootstrapFailureSchema = "carrier-lab-bootstrap-failure/v1"

type bootstrapRun struct {
	context         context.Context
	repositoryRoot  string
	goArchive       string
	seed            string
	faultFinalizer  bool
	stdout          io.Writer
	stderr          io.Writer
	tempRoot        string
	sessionRoot     string
	runID           string
	runDir          string
	evidenceDir     string
	containerName   string
	finalizerName   string
	inspectName     string
	networkName     string
	volumeName      string
	imageTag        string
	finalized       bool
	verifierStarted bool
}

type bootstrapFailureRecord struct {
	SchemaVersion string                   `json:"schema_version"`
	RunID         string                   `json:"run_id"`
	Status        string                   `json:"status"`
	Code          string                   `json:"code"`
	Stage         string                   `json:"stage"`
	Cleanup       bootstrapCleanupEvidence `json:"cleanup"`
}

type bootstrapCleanupEvidence struct {
	RunDirectoryAbsent bool `json:"run_directory_absent"`
	ContainersAbsent   bool `json:"owned_containers_absent"`
	NetworksAbsent     bool `json:"owned_networks_absent"`
	VolumesAbsent      bool `json:"owned_volumes_absent"`
}

// Bootstrap performs host-side setup for the pinned Carrier Lab verifier.
// Host Go launches this orchestration but is never recorded as success evidence;
// the verifier and candidate binary are rebuilt with the pinned archive.
func Bootstrap(ctx context.Context, repositoryRoot, goArchive, seed string, faultFinalizer bool, stdout, stderr io.Writer) error {
	run, err := newBootstrapRun(ctx, repositoryRoot, goArchive, seed, faultFinalizer, stdout, stderr)
	if err != nil {
		return err
	}
	defer func() {
		if !run.finalized {
			run.emergencyCleanup()
		}
	}()

	state, code, stage, err := run.inspectHost()
	if err != nil {
		return run.fail(code, stage, err)
	}
	if err := run.buildCandidateImage(); err != nil {
		return run.fail(reasonStageFailure, "build_carrier_lab_image", err)
	}
	carrierImageID, binarySHA256, err := run.inspectCandidateImage()
	if err != nil {
		return run.fail(reasonStageFailure, "inspect_carrier_lab_image", err)
	}
	state.carrierImageID = carrierImageID
	state.binarySHA256 = binarySHA256
	if err := run.removeCandidateImage(); err != nil {
		return run.fail(reasonStageFailure, "remove_carrier_lab_image", err)
	}
	if err := run.writeInput(state); err != nil {
		return run.fail(reasonStageFailure, "write_preflight_input", err)
	}
	if err := run.prepareWorkspace(); err != nil {
		return run.fail(reasonStageFailure, "prepare_workspace", err)
	}

	run.verifierStarted = true
	evaluateErr := run.runPinnedVerifier(binarySHA256)
	run.removeContainer(run.containerName)
	resources := run.resourcesAbsent()
	if run.faultFinalizer {
		resources.NetworksAbsent = false
	}
	if _, err := os.Stat(filepath.Join(run.runDir, "bin", "carrier-lab")); err != nil {
		return fmt.Errorf("pinned verifier produced no runnable binary: %w", err)
	}
	if _, err := os.Stat(filepath.Join(run.evidenceDir, manifestFilename)); err != nil {
		return fmt.Errorf("pinned verifier produced no canonical evidence: %w", err)
	}

	finalizerErr := run.runPinnedFinalizer(resources)
	run.removeContainer(run.finalizerName)
	postResources := run.resourcesAbsent()
	repeatedCleanup := run.safeRemoveRunDirectory() == nil
	run.finalized = finalizerErr == nil && repeatedCleanup && postResources.ContainersAbsent && postResources.NetworksAbsent && postResources.VolumesAbsent
	if evaluateErr == nil && run.finalized {
		fmt.Fprintln(run.stdout, "Carrier Lab preflight: passed")
		fmt.Fprintf(run.stdout, "Evidence: %s\n", run.evidenceDir)
		return nil
	}
	if evaluateErr != nil {
		fmt.Fprintf(run.stderr, "Carrier Lab preflight: preflight_failed (%v)\n", evaluateErr)
	} else {
		fmt.Fprintf(run.stderr, "Carrier Lab preflight: preflight_failed (%v)\n", finalizerErr)
	}
	fmt.Fprintf(run.stderr, "Evidence: %s\n", run.evidenceDir)
	return errors.Join(evaluateErr, finalizerErr, func() error {
		if run.finalized {
			return nil
		}
		return errors.New("outer cleanup was not proven complete")
	}())
}

func newBootstrapRun(ctx context.Context, repositoryRoot, goArchive, seed string, faultFinalizer bool, stdout, stderr io.Writer) (*bootstrapRun, error) {
	if ctx == nil || stdout == nil || stderr == nil {
		return nil, errors.New("bootstrap context and output writers are required")
	}
	root, err := canonicalDirectory(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("repository root: %w", err)
	}
	archive, err := canonicalRegularFile(goArchive)
	if err != nil {
		return nil, fmt.Errorf("go archive: %w", err)
	}
	if filepath.Base(archive) != expectedGoArchiveName {
		return nil, fmt.Errorf("go archive must be named %s", expectedGoArchiveName)
	}
	if strings.ContainsAny(root+archive, ",\r\n") {
		return nil, errors.New("repository and archive paths must not contain commas or line breaks")
	}
	if seed != "" && !runIDPattern.MatchString(seed) {
		return nil, errors.New("seed may contain only letters, digits, dot, underscore, and hyphen")
	}
	tempRoot, err := canonicalDirectory(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("system temporary directory: %w", err)
	}
	if pathsOverlap(tempRoot, root) {
		return nil, errors.New("system temporary directory intersects the repository")
	}
	prefix := sessionDirectoryPrefix + time.Now().UTC().Format("20060102T150405Z") + "."
	sessionRoot, err := os.MkdirTemp(tempRoot, prefix)
	if err != nil {
		return nil, fmt.Errorf("create isolated session: %w", err)
	}
	runID := strings.TrimPrefix(filepath.Base(sessionRoot), sessionDirectoryPrefix)
	if seed == "" {
		seed = runID
	}
	run := &bootstrapRun{
		context: ctx, repositoryRoot: root, goArchive: archive, seed: seed,
		faultFinalizer: faultFinalizer, stdout: stdout, stderr: stderr,
		tempRoot: tempRoot, sessionRoot: sessionRoot, runID: runID,
		runDir:      filepath.Join(sessionRoot, runDirectoryPrefix+runID),
		evidenceDir: filepath.Join(sessionRoot, evidenceDirectoryPrefix+runID),
	}
	resourceID := strings.NewReplacer(".", "-", "_", "-").Replace(runID)
	run.containerName = "ardents-carrier-preflight-" + resourceID
	run.finalizerName = run.containerName + "-finalizer"
	run.inspectName = run.containerName + "-inspect"
	run.networkName = run.containerName + "-network"
	run.volumeName = run.containerName + "-volume"
	run.imageTag = "ardents-carrier-lab-preflight:" + resourceID
	if err := os.Mkdir(run.runDir, 0o700); err != nil {
		_ = os.RemoveAll(sessionRoot)
		return nil, err
	}
	if err := os.Mkdir(run.evidenceDir, 0o700); err != nil {
		_ = os.RemoveAll(sessionRoot)
		return nil, err
	}
	if _, err := NewRunLayout(run.sessionRoot, run.repositoryRoot, run.tempRoot, run.runID); err != nil {
		_ = os.RemoveAll(sessionRoot)
		return nil, err
	}
	return run, nil
}

func (run *bootstrapRun) fail(code, stage string, cause error) error {
	run.emergencyCleanup()
	if !run.verifierStarted {
		record := bootstrapFailureRecord{
			SchemaVersion: bootstrapFailureSchema, RunID: run.runID, Status: "bootstrap_failed", Code: code, Stage: stage,
			Cleanup: bootstrapCleanupEvidence{
				RunDirectoryAbsent: pathAbsent(run.runDir), ContainersAbsent: run.containerAbsent(run.containerName) && run.containerAbsent(run.finalizerName) && run.containerAbsent(run.inspectName),
				NetworksAbsent: run.networkAbsent(), VolumesAbsent: run.volumeAbsent(),
			},
		}
		data, marshalErr := json.MarshalIndent(record, "", "  ")
		if marshalErr == nil {
			data = append(data, '\n')
			marshalErr = writeAtomic(filepath.Join(run.evidenceDir, "bootstrap-failure.json"), data, 0o600)
		}
		cause = errors.Join(cause, marshalErr)
	}
	fmt.Fprintf(run.stderr, "Carrier Lab bootstrap: bootstrap_failed (%s)\nEvidence: %s\n", code, run.evidenceDir)
	return fmt.Errorf("%s: %w", stage, cause)
}

func canonicalDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	resolved = filepath.Clean(resolved)
	if err := requireCanonicalDirectory(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func canonicalRegularFile(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", errors.New("path must be absolute")
	}
	clean := filepath.Clean(path)
	if err := requireNoSymlinkComponents(clean); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("path must name a regular file")
	}
	return resolved, nil
}

func hostPlatform() (string, string, string) {
	ubuntu := "unavailable"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VERSION_ID=") {
				ubuntu = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}
	return runtime.GOOS, runtime.GOARCH, ubuntu
}

func pathAbsent(path string) bool {
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

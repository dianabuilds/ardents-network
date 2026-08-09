package preflight

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Evaluate verifies one prepared Carrier Lab environment and writes preliminary
// evidence. FinalizeCleanup must be called before the result can pass.
func Evaluate(inputPath string, layout RunLayout) (Result, error) {
	if err := layout.validateOwnedPaths(true, false); err != nil {
		return Result{}, fmt.Errorf("validate run layout: %w", err)
	}
	if err := validateInputPath(inputPath, layout); err != nil {
		return Result{}, err
	}
	value, err := parseInputFile(inputPath)
	if err != nil {
		return Result{}, fmt.Errorf("read preflight input: %w", err)
	}
	return evaluate(value, layout, defaultRuntimeOptions())
}

func evaluate(value input, layout RunLayout, options runtimeOptions) (Result, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	if value.RunID != layout.runID {
		return Result{}, errors.New("input run ID does not match the run layout")
	}
	if err := prepareEvidenceDirectory(layout); err != nil {
		return Result{}, err
	}
	started := options.Now()
	record := manifest{
		SchemaVersion: manifestSchemaVersion, RunID: value.RunID, Seed: value.Seed,
		Source: sourceState{Revision: value.GitRevision, Dirty: value.GitDirty},
		Platform: platformState{HostOS: value.HostOS, HostArch: value.HostArch, HostUbuntuVersion: value.HostUbuntuVersion,
			ExecutionOS: options.ExecutionOS, ExecutionArch: options.ExecutionArch, UbuntuID: options.UbuntuID, UbuntuVersion: options.UbuntuVersion},
		Tools: value.Tools,
		Image: imageState{Reference: value.ImageReference, ExpectedManifestDigest: value.ExpectedImageManifestDigest,
			ObservedManifestDigest: value.ObservedImageManifestDigest, ImageID: value.ImageID,
			CarrierLabImageID: value.CarrierLabImageID},
		Binary: binaryState{SHA256: value.BinarySHA256},
		Toolchain: toolchainState{Archive: value.GoArchiveName, ExpectedArchiveSHA256: value.ExpectedGoArchiveSHA256,
			ObservedArchiveSHA256: value.ObservedGoArchiveSHA256, GoVersion: options.RuntimeGoVersion},
		Parameters: runParameters{RepositoryMount: value.RepositoryMount, ContainerNetwork: value.ContainerNetwork,
			GoProxy: value.GoProxy, GoCache: value.GoCache, GoModCache: value.GoModCache, RouteExecution: false},
	}
	record.Tools.Go = options.RuntimeGoVersion

	runStage := func(name string, check func() []failureReason) bool {
		stageStarted := elapsedNS(started, options.Now())
		var reasons []failureReason
		if options.FailStage == name {
			reasons = []failureReason{failure(reasonStageFailure, name, "injected preflight stage failure")}
		} else {
			reasons = check()
		}
		stage := stageResult{Name: name, StartedNS: stageStarted, FinishedNS: elapsedNS(started, options.Now()), Status: stagePassed}
		if len(reasons) > 0 {
			stage.Status = stageFailed
			stage.Reason = reasons[0].Message
			record.FailureReasons = append(record.FailureReasons, reasons...)
		}
		record.Stages = append(record.Stages, stage)
		return len(reasons) == 0
	}

	passed := runStage(stageValidateInput, func() []failureReason { return validateInput(value) })
	if passed {
		passed = runStage(stageVerifyPlatform, func() []failureReason { return verifyPlatform(value, options) })
	}
	if passed {
		passed = runStage(stageVerifyPinnedInputs, func() []failureReason { return verifyPinnedInputs(value) })
	}
	if passed {
		passed = runStage(stageVerifyRequiredTools, func() []failureReason { return verifyRuntimeTools(options) })
	}
	if passed {
		passed = runStage(stageWorkspaceIsolation, func() []failureReason { return verifyWorkspace(layout, value) })
	}
	if passed {
		passed = runStage(stageResourceCleanup, func() []failureReason { return exerciseResourceCleanup(layout.runDir) })
	}
	if passed {
		record.Status = statusChecksPassed
	} else {
		record.Status = statusPreflightFailed
	}
	if err := writeEvidence(layout, record, cleanupReport{
		SchemaVersion: cleanupSchemaVersion, RunID: value.RunID, Status: "pending_outer_cleanup",
	}); err != nil {
		return Result{Status: statusPreflightFailed, CleanupRequired: true}, err
	}
	return Result{Status: record.Status, ChecksPassed: record.Status == statusChecksPassed, CleanupRequired: true, Passed: false}, nil
}

func defaultRuntimeOptions() runtimeOptions {
	ubuntuID, ubuntuVersion := readOSRelease("/etc/os-release")
	return runtimeOptions{
		ExecutionOS: runtime.GOOS, ExecutionArch: runtime.GOARCH,
		UbuntuID: ubuntuID, UbuntuVersion: ubuntuVersion,
		RuntimeGoVersion: runtime.Version(), Now: time.Now,
	}
}

func verifyPlatform(_ input, options runtimeOptions) []failureReason {
	if options.ExecutionOS != "linux" || options.ExecutionArch != "amd64" || options.UbuntuID != "ubuntu" || options.UbuntuVersion != expectedUbuntuVersion {
		return []failureReason{failure(reasonUnsupportedPlatform, stageVerifyPlatform, "pinned execution image must be Ubuntu 26.04 LTS linux/amd64")}
	}
	return nil
}

func verifyPinnedInputs(value input) []failureReason {
	checks := []struct{ name, expected, declared, observed string }{
		{"image manifest", expectedImageManifestDigest, value.ExpectedImageManifestDigest, value.ObservedImageManifestDigest},
		{"Go archive", expectedGoArchiveSHA256, value.ExpectedGoArchiveSHA256, value.ObservedGoArchiveSHA256},
	}
	var reasons []failureReason
	for _, check := range checks {
		if check.declared != check.expected || check.observed != check.expected {
			reasons = append(reasons, failure(reasonDigestMismatch, stageVerifyPinnedInputs,
				fmt.Sprintf("%s digest does not match the R-013 pin", check.name)))
		}
	}
	if value.ImageReference != expectedImageReference {
		reasons = append(reasons, failure(reasonDigestMismatch, stageVerifyPinnedInputs, "image reference is not the immutable R-013 digest reference"))
	}
	if value.GoArchiveName != expectedGoArchiveName {
		reasons = append(reasons, failure(reasonDigestMismatch, stageVerifyPinnedInputs, "Go archive name does not match R-013"))
	}
	return reasons
}

func verifyRuntimeTools(options runtimeOptions) []failureReason {
	if options.RuntimeGoVersion != expectedGoVersion {
		return []failureReason{failure(reasonMissingRequiredTool, stageVerifyRequiredTools,
			fmt.Sprintf("pinned Go runtime must be %s, got %s", expectedGoVersion, options.RuntimeGoVersion))}
	}
	return nil
}

func verifyWorkspace(layout RunLayout, value input) []failureReason {
	if value.RepositoryMount != "read-only" || value.ContainerNetwork != "none" || value.GoProxy != "off" {
		return []failureReason{failure(reasonUnsafeWorkspace, stageWorkspaceIsolation, "repository must be read-only and the verifier must run with network and module downloads disabled")}
	}
	if err := layout.validateOwnedPaths(true, true); err != nil {
		return []failureReason{failure(reasonUnsafeWorkspace, stageWorkspaceIsolation, err.Error())}
	}
	cacheRoot := filepath.Join(layout.runDir, "cache")
	for _, cache := range []string{value.GoCache, value.GoModCache} {
		within, err := pathWithin(cache, cacheRoot)
		if err != nil || !within || samePath(cache, cacheRoot) {
			return []failureReason{failure(reasonUnsafeWorkspace, stageWorkspaceIsolation, "Go caches must be task-specific children of the disposable run directory")}
		}
	}
	return nil
}

func exerciseResourceCleanup(runDir string) []failureReason {
	resources := filepath.Join(runDir, "resources")
	if err := os.MkdirAll(resources, 0o700); err != nil {
		return []failureReason{failure(reasonStageFailure, stageResourceCleanup, err.Error())}
	}
	if err := os.WriteFile(filepath.Join(resources, "ephemeral-fixture"), []byte("synthetic and disposable\n"), 0o600); err != nil {
		return []failureReason{failure(reasonStageFailure, stageResourceCleanup, err.Error())}
	}
	if err := os.RemoveAll(resources); err != nil {
		return []failureReason{failure(reasonStageFailure, stageResourceCleanup, err.Error())}
	}
	if err := os.RemoveAll(resources); err != nil {
		return []failureReason{failure(reasonStageFailure, stageResourceCleanup, err.Error())}
	}
	return nil
}

func prepareEvidenceDirectory(layout RunLayout) error {
	if err := layout.validateOwnedPaths(true, false); err != nil {
		return fmt.Errorf("validate run layout before evidence write: %w", err)
	}
	if err := os.Mkdir(layout.evidenceDir, 0o700); err != nil && !os.IsExist(err) {
		return fmt.Errorf("create evidence directory: %w", err)
	}
	if err := layout.validateOwnedPaths(true, true); err != nil {
		return fmt.Errorf("validate evidence directory: %w", err)
	}
	return nil
}

func validateInputPath(path string, layout RunLayout) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("preflight input path must be absolute and canonical")
	}
	if filepath.Base(path) != "preflight-input.properties" {
		return errors.New("preflight input must use the owned filename")
	}
	within, err := pathWithin(path, layout.runDir)
	if err != nil || !within || samePath(path, layout.runDir) {
		return errors.New("preflight input is outside the owned run directory")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect preflight input: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("preflight input must be a regular file, not a symlink")
	}
	if err := requireNoSymlinkComponents(path); err != nil {
		return errors.New("preflight input path is not canonical")
	}
	return nil
}

func readOSRelease(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			values[key] = strings.Trim(strings.TrimSpace(value), "\"")
		}
	}
	return values["ID"], values["VERSION_ID"]
}

func elapsedNS(start, current time.Time) int64 {
	elapsed := current.Sub(start).Nanoseconds()
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func failure(code, stage, message string) failureReason {
	return failureReason{Code: code, Stage: stage, Message: message}
}

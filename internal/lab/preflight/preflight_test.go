package preflight

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestEvaluateWritesValidManifest(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)

	result, err := evaluate(fixture.input, fixture.layout, testRuntime())
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if result.Status != statusChecksPassed {
		t.Fatalf("status = %q, want %q", result.Status, statusChecksPassed)
	}
	record := loadManifest(t, fixture.layout.evidenceDir)
	if record.SchemaVersion != manifestSchemaVersion {
		t.Errorf("schema_version = %q", record.SchemaVersion)
	}
	if record.RunID != fixture.input.RunID || record.Seed != fixture.input.Seed {
		t.Errorf("run identity = (%q, %q)", record.RunID, record.Seed)
	}
	if record.Source.Revision != fixture.input.GitRevision || !record.Source.Dirty {
		t.Errorf("source = %#v", record.Source)
	}
	if record.Platform.ExecutionOS != "linux" || record.Platform.ExecutionArch != "amd64" || record.Platform.UbuntuVersion != expectedUbuntuVersion {
		t.Errorf("platform = %#v", record.Platform)
	}
	if record.Image.ObservedManifestDigest != expectedImageManifestDigest {
		t.Errorf("image = %#v", record.Image)
	}
	if record.Image.ImageID != fixture.input.ImageID || record.Image.CarrierLabImageID != fixture.input.CarrierLabImageID || record.Binary.SHA256 != fixture.input.BinarySHA256 {
		t.Errorf("build identity = image %#v binary %#v", record.Image, record.Binary)
	}
	if record.Toolchain.ObservedArchiveSHA256 != expectedGoArchiveSHA256 || record.Toolchain.GoVersion != expectedGoVersion {
		t.Errorf("toolchain = %#v", record.Toolchain)
	}
	if len(record.Stages) != 6 {
		t.Fatalf("got %d stages, want 6", len(record.Stages))
	}
	for _, stage := range record.Stages {
		if stage.FinishedNS < stage.StartedNS {
			t.Errorf("stage %q has non-monotonic timestamps", stage.Name)
		}
	}
}

func TestCanonicalVerdictCannotPassBeforeCleanup(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)

	result, err := evaluate(fixture.input, fixture.layout, testRuntime())
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if result.Passed || !result.ChecksPassed || !result.CleanupRequired {
		t.Fatalf("preliminary result = %#v", result)
	}
	var recorded verdict
	loadJSON(t, filepath.Join(fixture.layout.evidenceDir, verdictFilename), &recorded)
	if recorded.Status != statusChecksPassed {
		t.Fatalf("verdict status = %q, want intermediate %q", recorded.Status, statusChecksPassed)
	}
}

func TestNewRunLayoutDerivesOwnedPaths(t *testing.T) {
	t.Parallel()
	tempRoot := t.TempDir()
	runID := "20260809T120000Z-42"
	sessionRoot := filepath.Join(tempRoot, sessionDirectoryPrefix+runID)
	repositoryRoot := filepath.Join(tempRoot, "repository")
	for _, directory := range []string{sessionRoot, repositoryRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	layout, err := NewRunLayout(sessionRoot, repositoryRoot, tempRoot, runID)
	if err != nil {
		t.Fatalf("NewRunLayout() error = %v", err)
	}
	if layout.runDir != filepath.Join(sessionRoot, runDirectoryPrefix+runID) {
		t.Errorf("run directory = %q", layout.runDir)
	}
	if layout.evidenceDir != filepath.Join(sessionRoot, evidenceDirectoryPrefix+runID) {
		t.Errorf("evidence directory = %q", layout.evidenceDir)
	}
}

func TestContainerRunPathsKeepPinnedVerifierContract(t *testing.T) {
	t.Parallel()
	const sessionName = "carrier-lab-session-20260809T120000Z"
	want := "/carrier-lab/" + sessionName
	got := containerSessionPath(filepath.Join("host", "sessions", sessionName))
	if got != want {
		t.Fatalf("container session path = %q, want %q", got, want)
	}
	if containerTemporaryRoot != "/carrier-lab" {
		t.Fatalf("container temporary root changed: %q", containerTemporaryRoot)
	}
}

func TestValidateInputRejectsMissingRequiredField(t *testing.T) {
	t.Parallel()
	value := validInput()
	value.GitRevision = ""
	assertReasonCode(t, validateInput(value), reasonMissingRequiredField)
}

func TestValidateInputRejectsWrongSchemaVersion(t *testing.T) {
	t.Parallel()
	value := validInput()
	value.SchemaVersion = "carrier-lab-preflight-input/v999"
	assertReasonCode(t, validateInput(value), reasonInvalidSchemaVersion)
}

func TestEvaluateRejectsDigestMismatch(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	fixture.input.ObservedImageManifestDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	result, err := evaluate(fixture.input, fixture.layout, testRuntime())
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if result.Status != statusPreflightFailed {
		t.Fatalf("status = %q", result.Status)
	}
	assertReasonCode(t, loadManifest(t, fixture.layout.evidenceDir).FailureReasons, reasonDigestMismatch)
}

func TestEvaluateRejectsEvidenceOutsideOwnedSessionBeforeWrite(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	unsafe := fixture.layout
	unsafe.evidenceDir = filepath.Join(fixture.layout.tempRoot, evidenceDirectoryPrefix+fixture.input.RunID)

	if _, err := evaluate(fixture.input, unsafe, testRuntime()); err == nil {
		t.Fatal("evaluate() accepted an evidence directory outside the owned session")
	}
	if _, err := os.Stat(unsafe.evidenceDir); !os.IsNotExist(err) {
		t.Fatalf("unsafe evidence path was written: %v", err)
	}
}

func TestEvaluateRejectsUnsupportedOSOrArchitecture(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*runtimeOptions)
	}{
		{name: "os", edit: func(options *runtimeOptions) { options.ExecutionOS = "windows" }},
		{name: "architecture", edit: func(options *runtimeOptions) { options.ExecutionArch = "arm64" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newFixture(t)
			options := testRuntime()
			test.edit(&options)
			if _, err := evaluate(fixture.input, fixture.layout, options); err != nil {
				t.Fatalf("evaluate() error = %v", err)
			}
			assertReasonCode(t, loadManifest(t, fixture.layout.evidenceDir).FailureReasons, reasonUnsupportedPlatform)
		})
	}
}

func TestEvaluateAcceptsWindowsHostWithPinnedUbuntuContainer(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	fixture.input.HostOS = "windows"
	fixture.input.HostArch = "amd64"
	fixture.input.HostUbuntuVersion = "not-applicable"

	result, err := evaluate(fixture.input, fixture.layout, testRuntime())
	if err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if result.Status != statusChecksPassed {
		t.Fatalf("status = %q, want %q", result.Status, statusChecksPassed)
	}
}

func TestValidateInputRejectsMissingRequiredTool(t *testing.T) {
	t.Parallel()
	value := validInput()
	value.Tools.DockerServer = ""
	assertReasonCode(t, validateInput(value), reasonMissingRequiredTool)
}

func TestEvaluateRecordsStageFailure(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	options := testRuntime()
	options.FailStage = stageWorkspaceIsolation

	if _, err := evaluate(fixture.input, fixture.layout, options); err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	record := loadManifest(t, fixture.layout.evidenceDir)
	assertReasonCode(t, record.FailureReasons, reasonStageFailure)
	found := slices.ContainsFunc(record.Stages, func(stage stageResult) bool {
		return stage.Name == stageWorkspaceIsolation && stage.Status == stageFailed
	})
	if !found {
		t.Fatalf("workspace stage did not record its failure: %#v", record.Stages)
	}
}

func TestCleanupRunDirectoryIsIdempotent(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
		t.Fatal(err)
	}
	if _, err := FinalizeCleanup(fixture.layout, allResourcesAbsent()); err != nil {
		t.Fatal(err)
	}
	second, err := cleanupRunDirectory(fixture.layout, fixture.input.RunID)
	if err != nil {
		t.Fatalf("second cleanup: %v", err)
	}
	if second.Removed || !second.AlreadyAbsent {
		t.Fatalf("cleanup result = %#v", second)
	}
}

func TestEvaluateCreatesNothingInsideRepository(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	sentinel := filepath.Join(fixture.layout.repositoryRoot, "tracked.txt")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := tree(t, fixture.layout.repositoryRoot)

	if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}
	if after := tree(t, fixture.layout.repositoryRoot); !slices.Equal(before, after) {
		t.Fatalf("repository entries changed:\nbefore: %v\nafter: %v", before, after)
	}
}

func TestFinalizeCleanupUpdatesVerdict(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
		t.Fatalf("evaluate() error = %v", err)
	}

	result, err := FinalizeCleanup(fixture.layout, allResourcesAbsent())
	if err != nil {
		t.Fatalf("FinalizeCleanup() error = %v", err)
	}
	if result.Status != statusPassed {
		t.Fatalf("status = %q", result.Status)
	}
	record := loadManifest(t, fixture.layout.evidenceDir)
	if record.Cleanup == nil || !record.Cleanup.RunDirectoryRemoved || !record.Cleanup.RepeatedCleanupAlreadyAbsent {
		t.Fatalf("cleanup = %#v", record.Cleanup)
	}
	if _, err := os.Stat(fixture.layout.runDir); !os.IsNotExist(err) {
		t.Fatalf("run directory still exists: %v", err)
	}
}

func TestFinalizeCleanupRejectsCrossRunManifestBeforeDeletion(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
		t.Fatal(err)
	}
	record := loadManifest(t, fixture.layout.evidenceDir)
	record.RunID = "another-run"
	if err := writeJSON(filepath.Join(fixture.layout.evidenceDir, manifestFilename), record); err != nil {
		t.Fatal(err)
	}

	if _, err := FinalizeCleanup(fixture.layout, allResourcesAbsent()); err == nil {
		t.Fatal("FinalizeCleanup() accepted evidence from another run")
	}
	if _, err := os.Stat(fixture.layout.runDir); err != nil {
		t.Fatalf("cross-run mismatch removed the run directory: %v", err)
	}
}

func TestFinalizeCleanupRejectsUnsafeRunDirectoryBeforeDeletion(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(fixture.layout.repositoryRoot, "must-survive")
	if err := os.WriteFile(sentinel, []byte("owned by repository\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unsafe := fixture.layout
	unsafe.runDir = fixture.layout.repositoryRoot

	if _, err := FinalizeCleanup(unsafe, allResourcesAbsent()); err == nil {
		t.Fatal("FinalizeCleanup() accepted a repository path as the run directory")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "owned by repository\n" {
		t.Fatalf("repository sentinel changed: data=%q err=%v", data, err)
	}
}

func TestFinalizerFailureRemovesRunDirectoryAndCannotPass(t *testing.T) {
	t.Parallel()
	fixture := newFixture(t)
	if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
		t.Fatal(err)
	}
	resources := allResourcesAbsent()
	resources.NetworksAbsent = false

	result, err := FinalizeCleanup(fixture.layout, resources)
	if err != nil {
		t.Fatalf("FinalizeCleanup() error = %v", err)
	}
	if result.Passed || result.Status != statusPreflightFailed {
		t.Fatalf("finalizer failure result = %#v", result)
	}
	if _, err := os.Stat(fixture.layout.runDir); !os.IsNotExist(err) {
		t.Fatalf("failed finalizer left the run directory: %v", err)
	}
}

func TestSequentialRunsDoNotShareState(t *testing.T) {
	t.Parallel()
	tempRoot := t.TempDir()
	first := newFixtureAt(t, tempRoot, "20260809T120000Z-first")
	second := newFixtureAt(t, tempRoot, "20260809T120001Z-second")
	for _, fixture := range []fixture{first, second} {
		if _, err := evaluate(fixture.input, fixture.layout, testRuntime()); err != nil {
			t.Fatal(err)
		}
		if _, err := FinalizeCleanup(fixture.layout, allResourcesAbsent()); err != nil {
			t.Fatal(err)
		}
	}
	if samePath(first.layout.evidenceDir, second.layout.evidenceDir) {
		t.Fatal("sequential runs share an evidence directory")
	}
	for _, layout := range []RunLayout{first.layout, second.layout} {
		if _, err := os.Stat(layout.runDir); !os.IsNotExist(err) {
			t.Fatalf("run directory remains for %s: %v", layout.runID, err)
		}
		if _, err := os.Stat(filepath.Join(layout.evidenceDir, verdictFilename)); err != nil {
			t.Fatalf("run %s lost its own verdict: %v", layout.runID, err)
		}
	}
}

func TestBootstrapPinsInputsAndKeepsShellThin(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "scripts", "preflight.sh"))
	if err != nil {
		t.Fatalf("read preflight.sh: %v", err)
	}
	script := string(data)
	if lines := strings.Count(script, "\n"); lines > 20 {
		t.Errorf("preflight.sh must remain a thin launcher; got %d lines", lines)
	}
	for _, required := range []string{"go run", "GOTOOLCHAIN=local", "GOPROXY=off", "bootstrap"} {
		if !strings.Contains(script, required) {
			t.Errorf("preflight.sh does not contain launcher control %q", required)
		}
	}
	for _, forbidden := range []string{"docker ", "sha256sum ", "rm -rf", "preflight-manifest.json"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("preflight.sh owns substantive behavior %q", forbidden)
		}
	}

	var source strings.Builder
	for _, relative := range []string{
		filepath.Join("internal", "lab", "preflight", "evidence_model.go"),
		filepath.Join("internal", "lab", "preflight", "bootstrap.go"),
		filepath.Join("internal", "lab", "preflight", "bootstrap_docker.go"),
		filepath.Join("internal", "lab", "preflight", "bootstrap_environment.go"),
	} {
		part, readErr := os.ReadFile(filepath.Join(root, relative))
		if readErr != nil {
			t.Fatalf("read %s: %v", relative, readErr)
		}
		source.Write(part)
	}
	bootstrap := source.String()
	for _, required := range []string{
		expectedImageManifestDigest, expectedGoArchiveSHA256, "--pull=never", "none",
		"readonly", "GOTOOLCHAIN=local", "GOPROXY=off", "GOCACHE=", "GOMODCACHE=", "--rm",
		"MSYS_NO_PATHCONV=1", "finalizerName", "carrier-lab-bootstrap-failure/v1",
		"owned-networks-absent", "owned-volumes-absent",
	} {
		if !strings.Contains(bootstrap, required) {
			t.Errorf("Go bootstrap does not contain required control %q", required)
		}
	}
	for _, forbidden := range []string{"docker pull", "curl ", "ubuntu:26.04"} {
		if strings.Contains(bootstrap, forbidden) {
			t.Errorf("Go bootstrap contains forbidden automatic or mutable input %q", forbidden)
		}
	}
	if strings.Contains(bootstrap, "host must be Ubuntu") {
		t.Error("preflight must treat the Docker container, not the host OS, as the pinned Ubuntu environment")
	}
}

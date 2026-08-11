package preflight

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/lab/runlayout"
)

const (
	inputSchemaVersion    = "carrier-lab-preflight-input/v1"
	manifestSchemaVersion = "carrier-lab-preflight-manifest/v1"
	verdictSchemaVersion  = "carrier-lab-preflight-verdict/v1"
	cleanupSchemaVersion  = "carrier-lab-preflight-cleanup/v1"

	expectedUbuntuVersion       = "26.04"
	expectedImageManifestDigest = "sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
	expectedImageReference      = "ubuntu@" + expectedImageManifestDigest
	expectedGoArchiveName       = "go1.26.5.linux-amd64.tar.gz"
	expectedGoArchiveSHA256     = "5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053"
	expectedGoVersion           = "go1.26.5"

	runDirectoryPrefix      = runlayout.RunPrefix
	evidenceDirectoryPrefix = runlayout.EvidencePrefix
	sessionDirectoryPrefix  = runlayout.SessionPrefix

	manifestFilename = "preflight-manifest.json"
	verdictFilename  = "verdict.json"
	humanFilename    = "report.md"
	cleanupFilename  = "cleanup.json"

	statusChecksPassed    = "preflight_checks_passed"
	statusPassed          = "passed"
	statusPreflightFailed = "preflight_failed"

	stageValidateInput       = "validate_input"
	stageVerifyPlatform      = "verify_platform"
	stageVerifyPinnedInputs  = "verify_pinned_inputs"
	stageVerifyRequiredTools = "verify_required_tools"
	stageWorkspaceIsolation  = "verify_workspace_isolation"
	stageResourceCleanup     = "cleanup_ephemeral_resources"

	stagePassed = "passed"
	stageFailed = "failed"

	reasonMissingRequiredField = "missing_required_field"
	reasonInvalidSchemaVersion = "invalid_schema_version"
	reasonDigestMismatch       = "digest_mismatch"
	reasonUnsupportedPlatform  = "unsupported_platform"
	reasonMissingRequiredTool  = "missing_required_tool"
	reasonStageFailure         = "preflight_stage_failed"
	reasonUnsafeWorkspace      = "unsafe_workspace"
	reasonCleanupFailure       = "cleanup_failed"
)

type input struct {
	SchemaVersion               string
	RunID                       string
	Seed                        string
	GitRevision                 string
	GitDirty                    bool
	HostOS                      string
	HostArch                    string
	HostUbuntuVersion           string
	ImageReference              string
	ExpectedImageManifestDigest string
	ObservedImageManifestDigest string
	ImageID                     string
	CarrierLabImageID           string
	BinarySHA256                string
	GoArchiveName               string
	ExpectedGoArchiveSHA256     string
	ObservedGoArchiveSHA256     string
	RepositoryMount             string
	ContainerNetwork            string
	GoProxy                     string
	GoCache                     string
	GoModCache                  string
	Tools                       toolVersions
}

type toolVersions struct {
	Bash         string `json:"bash"`
	Git          string `json:"git"`
	DockerClient string `json:"docker_client"`
	DockerServer string `json:"docker_server"`
	SHA256Sum    string `json:"sha256sum"`
	Tar          string `json:"tar"`
	Go           string `json:"go"`
}

type runtimeOptions struct {
	ExecutionOS      string
	ExecutionArch    string
	UbuntuID         string
	UbuntuVersion    string
	RuntimeGoVersion string
	FailStage        string
	Now              func() time.Time
}

type manifest struct {
	SchemaVersion  string          `json:"schema_version"`
	RunID          string          `json:"run_id"`
	Seed           string          `json:"seed"`
	Source         sourceState     `json:"source"`
	Platform       platformState   `json:"platform"`
	Tools          toolVersions    `json:"tools"`
	Image          imageState      `json:"image"`
	Binary         binaryState     `json:"binary"`
	Toolchain      toolchainState  `json:"toolchain"`
	Parameters     runParameters   `json:"parameters"`
	Stages         []stageResult   `json:"stages"`
	Status         string          `json:"status"`
	FailureReasons []failureReason `json:"failure_reasons"`
	Cleanup        *cleanupState   `json:"cleanup,omitempty"`
}

type sourceState struct {
	Revision string `json:"git_revision"`
	Dirty    bool   `json:"dirty_worktree"`
}

type platformState struct {
	HostOS            string `json:"host_os"`
	HostArch          string `json:"host_arch"`
	HostUbuntuVersion string `json:"host_ubuntu_version"`
	ExecutionOS       string `json:"execution_os"`
	ExecutionArch     string `json:"execution_arch"`
	UbuntuID          string `json:"ubuntu_id"`
	UbuntuVersion     string `json:"ubuntu_version"`
}

type imageState struct {
	Reference              string `json:"reference"`
	ExpectedManifestDigest string `json:"expected_manifest_digest"`
	ObservedManifestDigest string `json:"observed_manifest_digest"`
	ImageID                string `json:"base_image_id"`
	CarrierLabImageID      string `json:"carrier_lab_image_id"`
}

type binaryState struct {
	SHA256 string `json:"sha256"`
}

type toolchainState struct {
	Archive               string `json:"archive"`
	ExpectedArchiveSHA256 string `json:"expected_archive_sha256"`
	ObservedArchiveSHA256 string `json:"observed_archive_sha256"`
	GoVersion             string `json:"go_version"`
}

type runParameters struct {
	RepositoryMount  string `json:"repository_mount"`
	ContainerNetwork string `json:"container_network"`
	GoProxy          string `json:"go_proxy"`
	GoCache          string `json:"go_cache"`
	GoModCache       string `json:"go_mod_cache"`
	RouteExecution   bool   `json:"route_execution"`
}

type stageResult struct {
	Name       string `json:"name"`
	StartedNS  int64  `json:"started_monotonic_ns"`
	FinishedNS int64  `json:"finished_monotonic_ns"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}

type failureReason struct {
	Code    string `json:"code"`
	Stage   string `json:"stage,omitempty"`
	Message string `json:"message"`
}

type cleanupState struct {
	OwnedContainersAbsent        bool `json:"owned_containers_absent"`
	OwnedNetworksAbsent          bool `json:"owned_networks_absent"`
	OwnedVolumesAbsent           bool `json:"owned_volumes_absent"`
	TemporaryCredentialsRemoved  bool `json:"temporary_credentials_removed"`
	GoCachesRemoved              bool `json:"go_caches_removed"`
	RunDirectoryRemoved          bool `json:"run_directory_removed"`
	RepeatedCleanupAlreadyAbsent bool `json:"repeated_cleanup_already_absent"`
	EvidenceRetained             bool `json:"evidence_retained"`
}

type verdict struct {
	SchemaVersion  string          `json:"schema_version"`
	RunID          string          `json:"run_id"`
	Status         string          `json:"status"`
	ManifestSHA256 string          `json:"manifest_sha256"`
	FailureReasons []failureReason `json:"failure_reasons"`
	Cleanup        *cleanupState   `json:"cleanup,omitempty"`
}

type cleanupReport struct {
	SchemaVersion string       `json:"schema_version"`
	RunID         string       `json:"run_id"`
	Status        string       `json:"status"`
	Cleanup       cleanupState `json:"cleanup"`
	Failure       string       `json:"failure,omitempty"`
}

// Result is the stable machine outcome returned to command callers. Passed is
// false for the intermediate checks-passed state.
type Result struct {
	Status          string
	ChecksPassed    bool
	CleanupRequired bool
	Passed          bool
}

type cleanupAttempt struct {
	Removed       bool
	AlreadyAbsent bool
}

// OwnedResources records the outer resources that must be absent before a run
// can receive a final passed verdict.
type OwnedResources struct {
	ContainersAbsent bool
	NetworksAbsent   bool
	VolumesAbsent    bool
}

package preflight

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type bootstrapState struct {
	gitRevision           string
	gitDirty              bool
	hostOS                string
	hostArch              string
	hostUbuntuVersion     string
	baseImageID           string
	carrierImageID        string
	binarySHA256          string
	observedArchiveSHA256 string
	gitVersion            string
	dockerClientVersion   string
	dockerServerVersion   string
	bashVersion           string
}

func (run *bootstrapRun) inspectHost() (bootstrapState, string, string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return bootstrapState{}, reasonMissingRequiredTool, "validate_host_environment", errors.New("git is unavailable")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return bootstrapState{}, reasonMissingRequiredTool, "validate_host_environment", errors.New("docker is unavailable")
	}
	archiveDigest, err := fileSHA256(run.goArchive)
	if err != nil {
		return bootstrapState{}, reasonMissingRequiredTool, "verify_go_archive", err
	}
	if archiveDigest != expectedGoArchiveSHA256 {
		return bootstrapState{}, reasonDigestMismatch, "verify_go_archive", fmt.Errorf("go archive digest mismatch: got %s", archiveDigest)
	}
	gitVersion, err := run.commandOutput("git", "--version")
	if err != nil {
		return bootstrapState{}, reasonMissingRequiredTool, "validate_host_environment", err
	}
	gitRevision, err := run.commandOutput("git", "-C", run.repositoryRoot, "rev-parse", "HEAD")
	if err != nil || gitRevision == "" {
		return bootstrapState{}, reasonMissingRequiredField, "inspect_source", errors.Join(errors.New("git revision is unavailable"), err)
	}
	gitStatus, err := run.commandOutput("git", "-C", run.repositoryRoot, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return bootstrapState{}, reasonStageFailure, "inspect_source", err
	}
	dockerVersions, err := run.commandOutput("docker", "version", "--format", "{{.Client.Version}}|{{.Server.Version}}")
	if err != nil {
		return bootstrapState{}, reasonMissingRequiredTool, "verify_docker", fmt.Errorf("docker client or daemon is unavailable: %w", err)
	}
	client, server, found := strings.Cut(dockerVersions, "|")
	if !found || client == "" || server == "" {
		return bootstrapState{}, reasonMissingRequiredTool, "verify_docker", errors.New("docker returned incomplete version information")
	}
	imagePlatform, err := run.commandOutput("docker", "image", "inspect", "--format", "{{.Os}}|{{.Architecture}}|{{.Id}}", expectedImageReference)
	if err != nil {
		return bootstrapState{}, reasonMissingRequiredTool, "verify_image", fmt.Errorf("pinned Ubuntu image is unavailable: %w", err)
	}
	parts := strings.Split(imagePlatform, "|")
	if len(parts) != 3 || parts[0] != "linux" || parts[1] != "amd64" || parts[2] == "" {
		return bootstrapState{}, reasonUnsupportedPlatform, "verify_image", fmt.Errorf("pinned image has unsupported identity %q", imagePlatform)
	}
	repoDigests, err := run.commandOutput("docker", "image", "inspect", "--format", "{{range .RepoDigests}}{{println .}}{{end}}", expectedImageReference)
	if err != nil || !linePresent(repoDigests, expectedImageReference) {
		return bootstrapState{}, reasonDigestMismatch, "verify_image", errors.New("pinned image manifest digest is not present locally")
	}
	hostOS, hostArch, hostUbuntu := hostPlatform()
	return bootstrapState{
		gitRevision: gitRevision, gitDirty: gitStatus != "", hostOS: hostOS, hostArch: hostArch,
		hostUbuntuVersion: hostUbuntu, baseImageID: parts[2], observedArchiveSHA256: archiveDigest,
		gitVersion: gitVersion, dockerClientVersion: client, dockerServerVersion: server,
		bashVersion: strings.TrimSpace(os.Getenv("ARDENTS_BOOTSTRAP_BASH_VERSION")),
	}, "", "", nil
}

func (run *bootstrapRun) writeInput(state bootstrapState) error {
	if state.bashVersion == "" {
		state.bashVersion = "unavailable launcher metadata"
	}
	containerSession := "/carrier-lab/" + filepath.Base(run.sessionRoot)
	containerRun := containerSession + "/" + filepath.Base(run.runDir)
	properties := []string{
		"schema_version=" + inputSchemaVersion,
		"run_id=" + run.runID,
		"seed=" + run.seed,
		"git_revision=" + state.gitRevision,
		"git_dirty=" + strconv.FormatBool(state.gitDirty),
		"host_os=" + state.hostOS,
		"host_arch=" + state.hostArch,
		"host_ubuntu_version=" + state.hostUbuntuVersion,
		"image_reference=" + expectedImageReference,
		"expected_image_manifest_digest=" + expectedImageManifestDigest,
		"observed_image_manifest_digest=" + expectedImageManifestDigest,
		"image_id=" + state.baseImageID,
		"carrier_lab_image_id=" + state.carrierImageID,
		"binary_sha256=" + state.binarySHA256,
		"go_archive_name=" + expectedGoArchiveName,
		"expected_go_archive_sha256=" + expectedGoArchiveSHA256,
		"observed_go_archive_sha256=" + state.observedArchiveSHA256,
		"repository_mount=read-only",
		"container_network=none",
		"go_proxy=off",
		"go_cache=" + containerRun + "/cache/go-build",
		"go_mod_cache=" + containerRun + "/cache/go-mod",
		"tool_bash=" + state.bashVersion,
		"tool_git=" + state.gitVersion,
		"tool_docker_client=" + state.dockerClientVersion,
		"tool_docker_server=" + state.dockerServerVersion,
		"tool_sha256sum=Go crypto/sha256 bootstrap; pinned container sha256sum",
	}
	for _, property := range properties {
		if strings.ContainsAny(property, "\r\n") {
			return errors.New("preflight property contains a line break")
		}
	}
	return os.WriteFile(filepath.Join(run.runDir, "preflight-input.properties"), []byte(strings.Join(properties, "\n")+"\n"), 0o600)
}

func (run *bootstrapRun) prepareWorkspace() error {
	for _, relative := range []string{"bin", "cache", "resources"} {
		if err := os.Mkdir(filepath.Join(run.runDir, relative), 0o700); err != nil {
			return err
		}
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func linePresent(value, wanted string) bool {
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == wanted {
			return true
		}
	}
	return false
}

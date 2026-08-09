package preflight

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification"
)

const pinnedVerifierScript = `
printf "%s  %s\n" "$1" "/input/$2" | sha256sum -c -
tar --version >"$3/tar.version"
IFS= read -r tar_version <"$3/tar.version"
printf "tool_tar=%s\n" "$tar_version" >>"$3/preflight-input.properties"
mkdir -p "$3/toolchain" "$3/cache/go-build" "$3/cache/go-mod" "$3/cache/go-tmp"
tar -C "$3/toolchain" -xzf "/input/$2"
actual_go_version=$("$3/toolchain/go/bin/go" version)
case "$actual_go_version" in
  "go version $4 linux/amd64") ;;
  *) printf "unexpected Go runtime: %s\n" "$actual_go_version" >&2; exit 2 ;;
esac
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off CGO_ENABLED=0
export GOCACHE="$3/cache/go-build" GOMODCACHE="$3/cache/go-mod" GOTMPDIR="$3/cache/go-tmp"
cd /repo
"$3/toolchain/go/bin/go" build -trimpath -buildvcs=false -ldflags=-buildid= -o "$3/bin/carrier-lab" ./cmd/carrier-lab
binary_sha256=$(sha256sum "$3/bin/carrier-lab")
binary_sha256=${binary_sha256%% *}
if [ "$binary_sha256" != "$8" ]; then
  printf "Carrier Lab binary hash differs from the reproducible image build\n" >&2
  exit 2
fi
"$3/bin/carrier-lab" evaluate \
  --input "$3/preflight-input.properties" \
  --repository-root /repo \
  --session-root "$5" \
  --temp-root "$6" \
  --run-id "$7"
`

func (run *bootstrapRun) buildCandidateImage() error {
	sourceSHA256, err := qualification.SourceSHA256(run.repositoryRoot)
	if err != nil {
		return fmt.Errorf("qualification source snapshot: %w", err)
	}
	return run.runCommand([]string{"DOCKER_BUILDKIT=1"}, "docker",
		"build", "--no-cache", "--pull=false", "--network=none",
		"--build-arg", "CARRIER_LAB_SOURCE_SHA256="+sourceSHA256,
		"--build-context", "go_archive="+filepath.Dir(run.goArchive),
		"--file", filepath.Join(run.repositoryRoot, "carrier-lab", "Dockerfile"),
		"--target", "application",
		"--tag", run.imageTag, run.repositoryRoot,
	)
}

func (run *bootstrapRun) inspectCandidateImage() (string, string, error) {
	imageID, err := run.commandOutput("docker", "image", "inspect", "--format", "{{.Id}}", run.imageTag)
	if err != nil {
		return "", "", err
	}
	binaryIdentity, err := run.commandOutput("docker", "run", "--rm", "--pull=never", "--name", run.inspectName,
		"--network", "none", "--read-only", "--entrypoint", "/usr/bin/sha256sum",
		run.imageTag, "/usr/local/bin/carrier-lab")
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(binaryIdentity)
	if len(fields) != 2 || len(fields[0]) != 64 {
		return "", "", fmt.Errorf("unexpected candidate binary identity %q", binaryIdentity)
	}
	return imageID, fields[0], nil
}

func (run *bootstrapRun) removeCandidateImage() error {
	return run.runCommand(nil, "docker", "image", "rm", "-f", run.imageTag)
}

func (run *bootstrapRun) runPinnedVerifier(expectedBinarySHA256 string) error {
	containerSession := "/carrier-lab/" + filepath.Base(run.sessionRoot)
	containerRun := containerSession + "/" + filepath.Base(run.runDir)
	containerTemp := "/carrier-lab"
	arguments := []string{
		"run", "--rm", "--pull=never", "--name", run.containerName, "--platform", "linux/amd64",
		"--network", "none", "--read-only",
		"--mount", bindMount(run.repositoryRoot, "/repo", true),
		"--mount", bindMount(run.sessionRoot, containerSession, false),
		"--mount", bindMount(run.goArchive, "/input/"+expectedGoArchiveName, true),
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=16m",
		expectedImageReference, "/bin/sh", "-ceu", pinnedVerifierScript, "carrier-preflight",
		expectedGoArchiveSHA256, expectedGoArchiveName, containerRun, expectedGoVersion,
		containerSession, containerTemp, run.runID, expectedBinarySHA256,
	}
	return run.runCommand(nil, "docker", arguments...)
}

func (run *bootstrapRun) runPinnedFinalizer(resources OwnedResources) error {
	containerSession := "/carrier-lab/" + filepath.Base(run.sessionRoot)
	containerRun := containerSession + "/" + filepath.Base(run.runDir)
	arguments := []string{
		"run", "--rm", "--pull=never", "--name", run.finalizerName, "--platform", "linux/amd64",
		"--network", "none", "--read-only",
		"--mount", bindMount(run.repositoryRoot, "/repo", true),
		"--mount", bindMount(run.sessionRoot, containerSession, false),
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=16m",
		expectedImageReference, containerRun + "/bin/carrier-lab", "finalize-cleanup",
		"--repository-root", "/repo", "--session-root", containerSession, "--temp-root", "/carrier-lab", "--run-id", run.runID,
		"--owned-containers-absent=" + strconv.FormatBool(resources.ContainersAbsent),
		"--owned-networks-absent=" + strconv.FormatBool(resources.NetworksAbsent),
		"--owned-volumes-absent=" + strconv.FormatBool(resources.VolumesAbsent),
	}
	return run.runCommand(nil, "docker", arguments...)
}

func bindMount(source, destination string, readOnly bool) string {
	value := "type=bind,src=" + source + ",dst=" + destination
	if readOnly {
		value += ",readonly"
	}
	return value
}

func (run *bootstrapRun) commandOutput(name string, arguments ...string) (string, error) {
	command := exec.CommandContext(run.context, name, arguments...)
	command.Env = bootstrapEnvironment(nil)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(output)), nil
}

func (run *bootstrapRun) runCommand(environment []string, name string, arguments ...string) error {
	command := exec.CommandContext(run.context, name, arguments...)
	command.Env = bootstrapEnvironment(environment)
	command.Stdout = run.stdout
	command.Stderr = run.stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func bootstrapEnvironment(additional []string) []string {
	environment := append([]string{}, os.Environ()...)
	environment = append(environment, "MSYS_NO_PATHCONV=1")
	return append(environment, additional...)
}

func (run *bootstrapRun) resourcesAbsent() OwnedResources {
	return OwnedResources{
		ContainersAbsent: run.containerAbsent(run.containerName) && run.containerAbsent(run.finalizerName) && run.containerAbsent(run.inspectName),
		NetworksAbsent:   run.networkAbsent(),
		VolumesAbsent:    run.volumeAbsent(),
	}
}

func (run *bootstrapRun) containerAbsent(name string) bool {
	return run.dockerInspectFails("container", name)
}

func (run *bootstrapRun) networkAbsent() bool {
	return run.dockerInspectFails("network", run.networkName)
}

func (run *bootstrapRun) volumeAbsent() bool {
	return run.dockerInspectFails("volume", run.volumeName)
}

func (run *bootstrapRun) dockerInspectFails(kind, name string) bool {
	command := exec.CommandContext(run.context, "docker", kind, "inspect", name)
	command.Env = bootstrapEnvironment(nil)
	output, err := command.CombinedOutput()
	if err == nil {
		return false
	}
	message := strings.ToLower(string(output))
	return strings.Contains(message, "no such") || strings.Contains(message, "not found")
}

func (run *bootstrapRun) removeContainer(name string) {
	run.ignoreDocker("rm", "-f", name)
}

func (run *bootstrapRun) emergencyCleanup() {
	for _, name := range []string{run.containerName, run.finalizerName, run.inspectName} {
		run.removeContainer(name)
	}
	run.ignoreDocker("network", "rm", run.networkName)
	run.ignoreDocker("volume", "rm", "-f", run.volumeName)
	run.ignoreDocker("image", "rm", "-f", run.imageTag)
	_ = run.safeRemoveRunDirectory()
}

func (run *bootstrapRun) ignoreDocker(arguments ...string) {
	if _, err := exec.LookPath("docker"); err != nil {
		return
	}
	command := exec.CommandContext(run.context, "docker", arguments...)
	command.Env = bootstrapEnvironment(nil)
	_ = command.Run()
}

func (run *bootstrapRun) safeRemoveRunDirectory() error {
	expected := filepath.Join(run.sessionRoot, runDirectoryPrefix+run.runID)
	if run.runDir != expected || pathsOverlap(run.runDir, run.repositoryRoot) {
		return errors.New("refusing unsafe run directory")
	}
	within, err := pathWithin(run.runDir, run.sessionRoot)
	if err != nil || !within || samePath(run.runDir, run.sessionRoot) {
		return errors.New("run directory is outside the owned session")
	}
	for range 2 {
		if err := os.RemoveAll(run.runDir); err != nil {
			return err
		}
		if !pathAbsent(run.runDir) {
			return errors.New("run directory remains after cleanup")
		}
	}
	return nil
}

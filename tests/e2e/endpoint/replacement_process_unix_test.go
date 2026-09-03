//go:build linux

package endpoint_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestEndpointReplaceRunsAuthenticatedCandidateAndFixedUserUnit(t *testing.T) {
	command := buildArdents(t)
	_, enrolled, input, keys, rootBytes := enrolledRuntimeBundleWithKeys(t, command)
	root, err := os.MkdirTemp("/tmp", "er-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeEndpointProcessTree(t, root) })
	environment := endpointEnvironment(root)
	runEnrolledUntilStopped(t, enrolled, input, environment)

	artifact, err := os.ReadFile(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	// ELF ignores a non-loadable trailing byte. Keeping executable behavior
	// identical lets this process test exercise a distinct authenticated target
	// digest and real atomic replacement without adding a test-only product
	// build flag.
	artifact = append(append([]byte(nil), artifact...), 0)
	platform := runtime.GOOS + "-" + runtime.GOARCH
	targetPath := "ardents/" + platform + "/endpoint"
	candidateMetadata := replacementMetadata(t, artifact, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys)
	candidateMetadata["2.root.json"] = replacementRoot(t, rootBytes, keys)
	candidate := replacementBundle(t, artifact, rootBytes, candidateMetadata, targetPath, platform)
	fakeBin, logPath := replacementSystemctlFixture(t)
	replace := exec.Command(enrolled, "endpoint", "replace", candidate)
	replace.Env = append(environment, "PATH="+fakeBin+":"+os.Getenv("PATH"), "ARDENTS_REPLACEMENT_SYSTEMCTL_LOG="+logPath)
	output, err := replace.CombinedOutput()
	if err != nil {
		t.Fatalf("endpoint replace: %v\n%s", err, output)
	}
	var result struct{ Kind, State string }
	if err := json.Unmarshal(output, &result); err != nil || result.Kind != "endpoint-replacement" || result.State != "committed-restart-permitted" {
		t.Fatalf("endpoint replace result = %q / %+v / %v", output, result, err)
	}
	log, err := os.ReadFile(logPath)
	if err != nil || string(log) != "--user stop ardents-endpoint.service\n--user start ardents-endpoint.service\n" {
		t.Fatalf("systemctl calls = %q, %v", log, err)
	}
	runEnrolledUntilStopped(t, enrolled, input, environment)
}

func TestEndpointRollbackUsesRetainedProgramAndFreshReleaseAuthorization(t *testing.T) {
	command := buildArdents(t)
	_, enrolled, input, keys, rootBytes := enrolledRuntimeBundleWithKeys(t, command)
	root, err := os.MkdirTemp("/tmp", "er-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeEndpointProcessTree(t, root) })
	environment := endpointEnvironment(root)
	runEnrolledUntilStopped(t, enrolled, input, environment)

	original, err := os.ReadFile(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	targetPath := "ardents/" + platform + "/endpoint"
	failedCandidate := []byte("#!/bin/sh\nexit 1\n")
	candidateMetadata := replacementMetadataVersion(t, failedCandidate, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys, 2, 2)
	candidateMetadata["2.root.json"] = replacementRootVersion(t, rootBytes, keys, 2)
	candidate := replacementBundle(t, failedCandidate, rootBytes, candidateMetadata, targetPath, platform)
	fakeBin, logPath := replacementSystemctlFixture(t)
	replace := exec.Command(enrolled, "endpoint", "replace", candidate)
	replace.Env = append(environment, "PATH="+fakeBin+":"+os.Getenv("PATH"), "ARDENTS_REPLACEMENT_SYSTEMCTL_LOG="+logPath)
	output, err := replace.CombinedOutput()
	if err == nil {
		t.Fatalf("endpoint replace unexpectedly accepted failed candidate:\n%s", output)
	}
	var failed struct {
		Kind            string `json:"Kind"`
		State           string `json:"State"`
		RecoveryProgram string `json:"recovery_program"`
	}
	firstLine := strings.SplitN(string(output), "\n", 2)[0]
	if err := json.Unmarshal([]byte(firstLine), &failed); err != nil || failed.Kind != "endpoint-replacement" ||
		failed.State != "rollback-authorization-required" || failed.RecoveryProgram == "" {
		t.Fatalf("failed replacement result = %q / %+v / %v", output, failed, err)
	}
	if log, readErr := os.ReadFile(logPath); readErr != nil || string(log) != "--user stop ardents-endpoint.service\n" {
		t.Fatalf("failed replacement systemctl calls = %q, %v", log, readErr)
	}

	rollbackMetadata := replacementMetadataVersion(t, original, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys, 3, 3)
	rollbackRoot := replacementRootVersion(t, rootBytes, keys, 2)
	rollbackMetadata["3.root.json"] = replacementRootVersion(t, rootBytes, keys, 3)
	rollbackBundle := replacementBundle(t, original, rollbackRoot, rollbackMetadata, targetPath, platform)
	rollback := exec.Command(failed.RecoveryProgram, "endpoint", "rollback", rollbackBundle)
	rollback.Env = append(environment, "PATH="+fakeBin+":"+os.Getenv("PATH"), "ARDENTS_REPLACEMENT_SYSTEMCTL_LOG="+logPath)
	output, err = rollback.CombinedOutput()
	if err != nil {
		t.Fatalf("endpoint rollback: %v\n%s", err, output)
	}
	var result struct{ Kind, State string }
	if err := json.Unmarshal(output, &result); err != nil || result.Kind != "endpoint-rollback" || result.State != "rollback-committed-restart-permitted" {
		t.Fatalf("endpoint rollback result = %q / %+v / %v", output, result, err)
	}
	if log, readErr := os.ReadFile(logPath); readErr != nil || string(log) != "--user stop ardents-endpoint.service\n--user stop ardents-endpoint.service\n--user start ardents-endpoint.service\n" {
		t.Fatalf("rollback systemctl calls = %q, %v", log, readErr)
	}
	if restored, readErr := os.ReadFile(enrolled); readErr != nil || !bytes.Equal(restored, original) {
		t.Fatalf("restored program = %d bytes / %v", len(restored), readErr)
	}
	runEnrolledUntilStopped(t, enrolled, input, environment)
}

func replacementRoot(t *testing.T, rootBytes []byte, keys []enrolledRuntimeKey) []byte {
	return replacementRootVersion(t, rootBytes, keys, 2)
}

func replacementRootVersion(t *testing.T, rootBytes []byte, keys []enrolledRuntimeKey, version int64) []byte {
	t.Helper()
	root := metadata.Root()
	if _, err := root.FromBytes(rootBytes); err != nil {
		t.Fatal(err)
	}
	root.Signed.Version = version
	root.ClearSignatures()
	enrolledRuntimeSign(t, root, keys, 5)
	encoded, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func endpointEnvironment(root string) []string {
	return append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(root, "config"), "XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"), "XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"))
}

func runEnrolledUntilStopped(t *testing.T, command, input string, environment []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	running := exec.CommandContext(ctx, command, "endpoint", "enroll", input)
	running.Env = environment
	stdout, err := running.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &processStderrBuffer{}
	running.Stderr = stderr
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if finished {
			return
		}
		if err := running.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("terminate Endpoint after failed process assertion: %v", err)
		}
		if err := running.Wait(); err != nil {
			t.Errorf("join Endpoint after failed process assertion: %v", err)
		}
	})
	scanner := bufio.NewScanner(stdout)
	ready := false
	for scanner.Scan() {
		var event struct{ Kind, State string }
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind == "endpoint-lifecycle" && event.State == "ready" {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("Endpoint did not become ready: %v; %s", scanner.Err(), stderr.String())
	}
	if err := running.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatalf("Endpoint did not report stopped: %v; %s", scanner.Err(), stderr.String())
	}
	waitErr := running.Wait()
	finished = true
	if waitErr != nil {
		t.Fatalf("Endpoint exit: %v; %s", waitErr, stderr.String())
	}
}

func removeEndpointProcessTree(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Errorf("remove process fixture %s: %v", path, err)
		return
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("process fixture remains at %s: %v", path, err)
	}
}

func replacementMetadata(t *testing.T, artifact []byte, targetPath, platform string, now time.Time, keys []enrolledRuntimeKey) map[string][]byte {
	return replacementMetadataVersion(t, artifact, targetPath, platform, now, keys, 2, 2)
}

func replacementMetadataVersion(t *testing.T, artifact []byte, targetPath, platform string, now time.Time, keys []enrolledRuntimeKey, metadataVersion, releaseVersion int64) map[string][]byte {
	t.Helper()
	expires := now.Add(time.Hour)
	digest := sha256.Sum256(artifact)
	custom, err := json.Marshal(map[string]any{
		"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": platform, "architecture": runtime.GOARCH,
		"environment": "alpha", "network": "alpha-network-1", "release_identity": "ardents-alpha-" + strconv.FormatInt(releaseVersion, 10), "release_version": releaseVersion,
		"source_revision": "test-source-v2", "build_input_commitment": "test-inputs-v2", "build_identity": "test-build-v2",
		"dependency_identity": "test-dependencies", "sbom_identity": "test-sbom", "attestation_policy": "two-builder",
		"qualification": "qualified", "build_state": "current", "protocol_phase": "required", "protocol_overlapped_since": now.Add(-100 * 24 * time.Hour),
		"capacity_ready": true, "drain_ready": true, "build_safety_no_new_work_after": now.Add(20 * time.Minute), "build_safety_terminate_after": now.Add(40 * time.Minute),
		"builder_attestations": []map[string]string{
			{"builder_identity": "builder-a", "build_identity": "test-build-v2", "source_revision": "test-source-v2", "build_input_commitment": "test-inputs-v2", "target_sha256": hex.EncodeToString(digest[:])},
			{"builder_identity": "builder-b", "build_identity": "test-build-v2", "source_revision": "test-source-v2", "build_input_commitment": "test-inputs-v2", "target_sha256": hex.EncodeToString(digest[:])},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	targets := metadata.Targets(expires)
	targets.Signed.Version = metadataVersion
	targets.Signed.Targets[targetPath] = &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: targetPath, Custom: (*json.RawMessage)(&custom)}
	enrolledRuntimeSign(t, targets, keys, 3)
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetsDigest := sha256.Sum256(targetsBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Version = metadataVersion
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: metadataVersion, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsDigest[:]}}
	enrolledRuntimeSign(t, snapshot, keys, 3)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Version = metadataVersion
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: metadataVersion, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	enrolledRuntimeSign(t, timestamp, keys, 3)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	version := strconv.FormatInt(metadataVersion, 10)
	return map[string][]byte{"timestamp.json": timestampBytes, version + ".snapshot.json": snapshotBytes, version + ".targets.json": targetsBytes}
}

func replacementBundle(t *testing.T, artifact, root []byte, metadataFiles map[string][]byte, targetPath, platform string) string {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "replacement")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	artifactName := "ardents-" + platform
	descriptor := strings.Join([]string{"schema=ardents-offline-replacement-bundle-v1", "target_path=" + targetPath,
		"artifact=" + artifactName, "trusted_root=1.root.json", "platform=" + platform, "architecture=" + runtime.GOARCH,
		"environment=alpha", "network=alpha-network-1"}, "\n") + "\n"
	files := map[string][]byte{"REPLACEMENT": []byte(descriptor), artifactName: artifact, "1.root.json": root}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	for name, contents := range files {
		mode := os.FileMode(0o600)
		if name == artifactName {
			mode = 0o700
		}
		writeEnrollmentFile(t, filepath.Join(bundle, name), contents, mode)
	}
	return bundle
}

func replacementSystemctlFixture(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	log := filepath.Join(directory, "systemctl.log")
	script := filepath.Join(directory, "systemctl")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ARDENTS_REPLACEMENT_SYSTEMCTL_LOG\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return directory, log
}

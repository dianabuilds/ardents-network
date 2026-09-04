//go:build linux

package endpoint_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestEnrolledPortableAcceptsPinnedBundleAndReleaseDecision(t *testing.T) {
	command := buildArdents(t)
	bundle, enrolledCommand, legacyInput := enrolledRuntimeBundle(t, command)
	manifestPin := enrolledRuntimeManifestPin(t, bundle)
	root := enrolledRuntimeRoot(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	running := exec.CommandContext(ctx, enrolledCommand, "endpoint", "enroll", bundle, manifestPin)
	running.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"),
	)
	stdout, err := running.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &processStderrBuffer{}
	running.Stderr = stderr
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	var events []struct {
		Kind, State, Outcome string
		Attachment           string
	}
	for len(events) < 3 && scanner.Scan() {
		var event struct {
			Kind, State, Outcome string
			Attachment           string
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("decode enrolled Portable event %q: %v", scanner.Text(), err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read enrolled Portable events: %v", err)
	}
	if len(events) != 3 || events[0].Kind != "endpoint-lifecycle" || events[0].State != "starting" ||
		events[1].Kind != "release-decision" || (events[1].Outcome != "release-accepted" && events[1].Outcome != "no-update") ||
		events[2].Kind != "endpoint-lifecycle" || events[2].State != "ready" || events[2].Attachment == "" {
		t.Fatalf("enrolled Portable startup events = %+v; stderr=%s", events, stderr.String())
	}
	if info, err := os.Lstat(events[2].Attachment); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("enrolled Portable attachment = %v / %v", info, err)
	}
	if err := running.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatalf("enrolled Portable did not report stopped: %v; stderr=%s", scanner.Err(), stderr.String())
	}
	var stopped struct {
		Kind, State string
	}
	if err := json.Unmarshal(scanner.Bytes(), &stopped); err != nil || stopped.Kind != "endpoint-lifecycle" || stopped.State != "stopped" {
		t.Fatalf("enrolled Portable stop event = %q / %+v / %v", scanner.Text(), stopped, err)
	}
	if err := running.Wait(); err != nil {
		t.Fatalf("enrolled Portable exit: %v\n%s", err, stderr.String())
	}
	if _, err := os.Lstat(events[2].Attachment); !os.IsNotExist(err) {
		t.Fatalf("enrolled Portable attachment remains after stop: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "state", "ardents", "floors", "release-decision", "current")); err != nil {
		t.Fatalf("release floors were not committed: %v", err)
	}
	if _, err := os.Stat(bundle); err != nil {
		t.Fatal(err)
	}
	// The successor-start path is deliberately independent of the now-retired
	// first bundle: it accepts only the durable selected-program record written
	// during the first verified enrollment. Moving the exact same bytes out of
	// the bundle lets the test prove that a routine start does not silently
	// rerun (or weaken) the first-bundle check.
	successor := filepath.Join(t.TempDir(), "ardents-successor")
	if err := os.Rename(enrolledCommand, successor); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(bundle); err != nil {
		t.Fatal(err)
	}
	// The persisted v1 user unit still passes its one JSON argument. The old
	// bundle is gone, so this restart proves that an accepted replacement is
	// recognized before any obsolete first-enrollment verification is attempted.
	restarted := exec.CommandContext(ctx, successor, "endpoint", "enroll", legacyInput)
	restarted.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"),
	)
	restartedOut, err := restarted.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(); err != nil {
		t.Fatal(err)
	}
	restartedFinished := false
	t.Cleanup(func() {
		if restartedFinished {
			return
		}
		if err := restarted.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("terminate successor Endpoint after failed process assertion: %v", err)
		}
		if err := restarted.Wait(); err != nil {
			t.Errorf("join successor Endpoint after failed process assertion: %v", err)
		}
	})
	restartedScanner := bufio.NewScanner(restartedOut)
	var restartedEvents []struct{ Kind, State string }
	for len(restartedEvents) < 3 && restartedScanner.Scan() {
		var event struct{ Kind, State string }
		if err := json.Unmarshal(restartedScanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		restartedEvents = append(restartedEvents, event)
	}
	if len(restartedEvents) != 3 || restartedEvents[0].State != "starting" ||
		restartedEvents[1].Kind != "endpoint-replacement" || restartedEvents[1].State != "current" ||
		restartedEvents[2].State != "ready" {
		t.Fatalf("successor restart events = %+v", restartedEvents)
	}
	if err := restarted.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if !restartedScanner.Scan() {
		t.Fatalf("successor restart did not stop: %v", restartedScanner.Err())
	}
	if err := restarted.Wait(); err != nil {
		restartedFinished = true
		t.Fatalf("successor restart exit: %v", err)
	}
	restartedFinished = true
}

func TestEnrolledPortableReportsInvalidPinBeforeReady(t *testing.T) {
	command := buildArdents(t)
	bundle, enrolledCommand, _ := enrolledRuntimeBundle(t, command)
	manifestPin := enrolledRuntimeManifestPin(t, bundle)
	if err := os.WriteFile(filepath.Join(bundle, "SHA256SUMS"), []byte("changed-before-parse\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := enrolledRuntimeRoot(t)
	running := exec.Command(enrolledCommand, "endpoint", "enroll", bundle, manifestPin)
	running.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"),
	)
	output, err := running.CombinedOutput()
	if err == nil {
		t.Fatalf("changed enrollment pin unexpectedly started: %s", output)
	}
	var events []struct {
		Kind, State, Reason string
	}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		var event struct {
			Kind, State, Reason string
		}
		if json.Unmarshal(line, &event) == nil && event.Kind == "endpoint-lifecycle" {
			events = append(events, event)
		}
	}
	if len(events) != 2 || events[0].State != "starting" || events[1].State != "incompatible" ||
		events[1].Reason != "alpha-enrollment-invalid" {
		t.Fatalf("changed enrollment lifecycle = %+v; output=%s", events, output)
	}
	if bytes.Contains(output, []byte("\"state\":\"ready\"")) {
		t.Fatalf("changed enrollment reached ready: %s", output)
	}
}

func TestEnrolledPortableRejectsInventoryWithoutHeadlessCompanions(t *testing.T) {
	command := buildArdents(t)
	bundle, enrolledCommand, manifestPin := enrollmentBundle(t, command)
	root := enrolledRuntimeRoot(t)
	running := exec.Command(enrolledCommand, "endpoint", "enroll", bundle, manifestPin)
	running.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"),
	)
	output, err := running.CombinedOutput()
	if err == nil {
		t.Fatalf("incomplete headless inventory unexpectedly started: %s", output)
	}
	var events []struct {
		Kind, State, Reason string
	}
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		var event struct {
			Kind, State, Reason string
		}
		if json.Unmarshal(line, &event) == nil && event.Kind == "endpoint-lifecycle" {
			events = append(events, event)
		}
	}
	if len(events) != 2 || events[0].State != "starting" || events[1].State != "incompatible" ||
		events[1].Reason != "alpha-enrollment-invalid" || bytes.Contains(output, []byte("\"state\":\"ready\"")) {
		t.Fatalf("incomplete headless inventory lifecycle = %+v; output=%s", events, output)
	}
}

func enrolledRuntimeRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "ae-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

type enrolledRuntimeKey struct {
	public ed25519.PublicKey
	signer signature.Signer
}

func enrolledRuntimeBundle(t *testing.T, command string) (string, string, string) {
	bundle, enrolled, input, _, _ := enrolledRuntimeBundleWithKeys(t, command)
	return bundle, enrolled, input
}

func enrolledRuntimeManifestPin(t *testing.T, bundle string) string {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join(bundle, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(manifest)
	return hex.EncodeToString(pin[:])
}

func enrolledRuntimeBundleWithKeys(t *testing.T, command string) (string, string, string, []enrolledRuntimeKey, []byte) {
	t.Helper()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	artifactName := "ardents-" + platform
	controlName := "ardents-control-" + platform
	nodeName := "ardents-node-" + platform
	custodyName := "ardents-custody-" + platform
	artifact, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := "ardents/" + platform + "/endpoint"
	now := time.Now().UTC().Truncate(time.Second)
	metadataFiles, rootBytes, keys := enrolledRuntimeMetadataWithKeys(t, artifact, targetPath, platform, now)
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v3", "cohort=closed-cohort-1", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json", "control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1", "control_compatibility=compatibility.ac1", "control_release_root=release.pub", "control_network_root=network.pub", "control_compatibility_root=compatibility.pub", "corpus_authority=corpus.pub", "control_artifact=" + controlName,
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact, controlName: []byte("control"), nodeName: []byte("node"), custodyName: []byte("custody"), "catalog.ac1": []byte("catalog"), "catalog.pub": []byte("key"), "release.ac1": []byte("release control"), "network.ac1": []byte("network control"), "compatibility.ac1": []byte("compatibility control"), "release.pub": []byte("release key"), "network.pub": []byte("network key"), "compatibility.pub": []byte("compatibility key"), "corpus.pub": []byte("corpus")}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	names := []string{"1.root.json", "1.snapshot.json", "1.targets.json", "RELEASE", artifactName, controlName, nodeName, custodyName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "corpus.pub", "timestamp.json"}
	for _, name := range names {
		mode := os.FileMode(0o600)
		if name == artifactName {
			mode = 0o700
		}
		writeEnrollmentFile(t, filepath.Join(bundle, name), files[name], mode)
	}
	manifest := enrolledRuntimeManifest(t, names, files)
	writeEnrollmentFile(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	pin := sha256.Sum256(manifest)
	input := filepath.Join(t.TempDir(), "alpha-enrollment.json")
	raw, err := json.Marshal(map[string]string{
		"schema": "ardents-alpha-enrollment-input-v1", "bundle_root": bundle, "cohort": "closed-cohort-1", "release": "alpha-1",
		"platform": platform, "manifest_sha256": hex.EncodeToString(pin[:]), "environment": "alpha", "network": "alpha-network-1", "target_path": targetPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, input, raw, 0o600)
	return bundle, filepath.Join(bundle, artifactName), input, keys, rootBytes
}

func enrolledRuntimeMetadata(t *testing.T, artifact []byte, targetPath, platform string, now time.Time) (map[string][]byte, []byte) {
	// The alpha-control fixture selects a completed required-protocol overlap so
	// its first fresh alpha-control observation is exactly release-accepted.
	metadataFiles, rootBytes, _ := enrolledRuntimeMetadataWithKeysAt(t, artifact, targetPath, platform, now, now.Add(-100*24*time.Hour))
	return metadataFiles, rootBytes
}

func enrolledRuntimeMetadataWithKeys(t *testing.T, artifact []byte, targetPath, platform string, now time.Time) (map[string][]byte, []byte, []enrolledRuntimeKey) {
	return enrolledRuntimeMetadataWithKeysAt(t, artifact, targetPath, platform, now, now.Add(-48*time.Hour))
}

func enrolledRuntimeMetadataWithKeysAt(t *testing.T, artifact []byte, targetPath, platform string, now, protocolOverlappedSince time.Time) (map[string][]byte, []byte, []enrolledRuntimeKey) {
	t.Helper()
	keys := enrolledRuntimeKeys(t)
	expires := now.Add(time.Hour)
	root := metadata.Root(expires)
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1",
		"ardents_environment": "alpha", "ardents_network": "alpha-network-1"}
	keyIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		public, err := metadata.KeyFromPublicKey(key.public)
		if err != nil {
			t.Fatal(err)
		}
		id, err := public.ID()
		if err != nil {
			t.Fatal(err)
		}
		root.Signed.Keys[id] = public
		keyIDs = append(keyIDs, id)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: 3}
	}
	enrolledRuntimeSign(t, root, keys, 5)
	rootBytes, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(artifact)
	custom, err := json.Marshal(map[string]any{
		"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": platform, "architecture": runtime.GOARCH,
		"environment": "alpha", "network": "alpha-network-1", "release_identity": "ardents-alpha-1", "release_version": 1,
		"source_revision": "test-source", "build_input_commitment": "test-inputs", "build_identity": "test-build",
		"dependency_identity": "test-dependencies", "sbom_identity": "test-sbom", "attestation_policy": "two-builder",
		"qualification": "qualified", "build_state": "current", "protocol_phase": "required", "protocol_overlapped_since": protocolOverlappedSince,
		"capacity_ready": true, "drain_ready": true, "build_safety_no_new_work_after": now.Add(20 * time.Minute), "build_safety_terminate_after": now.Add(40 * time.Minute),
		"builder_attestations": []map[string]string{
			{"builder_identity": "builder-a", "build_identity": "test-build", "source_revision": "test-source", "build_input_commitment": "test-inputs", "target_sha256": hex.EncodeToString(digest[:])},
			{"builder_identity": "builder-b", "build_identity": "test-build", "source_revision": "test-source", "build_input_commitment": "test-inputs", "target_sha256": hex.EncodeToString(digest[:])},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	targets := metadata.Targets(expires)
	targets.Signed.Targets[targetPath] = &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: targetPath, Custom: (*json.RawMessage)(&custom)}
	enrolledRuntimeSign(t, targets, keys, 3)
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetsDigest := sha256.Sum256(targetsBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsDigest[:]}}
	enrolledRuntimeSign(t, snapshot, keys, 3)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	enrolledRuntimeSign(t, timestamp, keys, 3)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"timestamp.json": timestampBytes, "1.snapshot.json": snapshotBytes, "1.targets.json": targetsBytes}, rootBytes, keys
}

func enrolledRuntimeKeys(t *testing.T) []enrolledRuntimeKey {
	t.Helper()
	keys := make([]enrolledRuntimeKey, 0, 5)
	for range 5 {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signature.LoadSigner(private, crypto.Hash(0))
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, enrolledRuntimeKey{public: public, signer: signer})
	}
	return keys
}

func enrolledRuntimeSign(t *testing.T, value interface {
	Sign(signature.Signer) (*metadata.Signature, error)
}, keys []enrolledRuntimeKey, count int) {
	t.Helper()
	for _, key := range keys[:count] {
		if _, err := value.Sign(key.signer); err != nil {
			t.Fatal(err)
		}
	}
}

func enrolledRuntimeManifest(t *testing.T, names []string, files map[string][]byte) []byte {
	t.Helper()
	canonicalNames := append([]string(nil), names...)
	sort.Strings(canonicalNames)
	lines := make([]string, 0, len(canonicalNames))
	for _, name := range canonicalNames {
		contents, found := files[name]
		if !found {
			t.Fatalf("missing enrolled runtime file %q", name)
		}
		digest := sha256.Sum256(contents)
		lines = append(lines, hex.EncodeToString(digest[:])+"  "+name)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

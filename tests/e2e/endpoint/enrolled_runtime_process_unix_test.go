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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestEnrolledPortableAcceptsPinnedBundleAndReleaseDecision(t *testing.T) {
	command := buildArdents(t)
	bundle, enrolledCommand, input := enrolledRuntimeBundle(t, command)
	root, err := os.MkdirTemp("/tmp", "ae-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	running := exec.CommandContext(ctx, enrolledCommand, "endpoint", "enroll", input)
	running.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+filepath.Join(root, "config"),
		"XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_CACHE_HOME="+filepath.Join(root, "cache"),
		"XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"),
	)
	var stderr bytes.Buffer
	running.Stderr = &stderr
	stdout, err := running.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
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
}

type enrolledRuntimeKey struct {
	public ed25519.PublicKey
	signer signature.Signer
}

func enrolledRuntimeBundle(t *testing.T, command string) (string, string, string) {
	t.Helper()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	artifactName := "ardents-" + platform
	artifact, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := "ardents/" + platform + "/endpoint"
	now := time.Now().UTC().Truncate(time.Second)
	metadataFiles, rootBytes := enrolledRuntimeMetadata(t, artifact, targetPath, platform, now)
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v1", "cohort=closed-cohort-1", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json",
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	names := []string{"1.root.json", "1.snapshot.json", "1.targets.json", "RELEASE", artifactName, "timestamp.json"}
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
	return bundle, filepath.Join(bundle, artifactName), input
}

func enrolledRuntimeMetadata(t *testing.T, artifact []byte, targetPath, platform string, now time.Time) (map[string][]byte, []byte) {
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
		"qualification": "qualified", "build_state": "current", "protocol_phase": "required", "protocol_overlapped_since": now.Add(-48 * time.Hour),
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
	return map[string][]byte{"timestamp.json": timestampBytes, "1.snapshot.json": snapshotBytes, "1.targets.json": targetsBytes}, rootBytes
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
	lines := make([]string, 0, len(names))
	for _, name := range names {
		contents, found := files[name]
		if !found {
			t.Fatalf("missing enrolled runtime file %q", name)
		}
		digest := sha256.Sum256(contents)
		lines = append(lines, hex.EncodeToString(digest[:])+"  "+name)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

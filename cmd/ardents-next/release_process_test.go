package main

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// The fixture signs actual compiled consumer bytes with ephemeral test keys.
// Its inventory and custom fields are encoded independently of product codecs.
func signedConsumerBundle(t *testing.T) (string, string, string) {
	t.Helper()
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	program := "ardents-" + platform
	control := "ardents-control-" + platform
	if runtime.GOOS == "windows" {
		program += ".exe"
		control += ".exe"
	}
	ref := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	expires := ref.Add(365 * 24 * time.Hour)
	root := metadata.Root(expires)
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": "h3-test", "ardents_network": "new-release-test"}
	var signers []signature.Signer
	var ids []string
	for range 5 {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signature.LoadSigner(private, crypto.Hash(0))
		if err != nil {
			t.Fatal(err)
		}
		key, err := metadata.KeyFromPublicKey(public)
		if err != nil {
			t.Fatal(err)
		}
		id, err := key.ID()
		if err != nil {
			t.Fatal(err)
		}
		root.Signed.Keys[id] = key
		ids = append(ids, id)
		signers = append(signers, signer)
	}
	for _, role := range []string{"root", "timestamp", "snapshot", "targets"} {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), ids...), Threshold: 3}
	}
	sign := func(value interface {
		Sign(signature.Signer) (*metadata.Signature, error)
	}) {
		for _, signer := range signers[:3] {
			if _, err := value.Sign(signer); err != nil {
				t.Fatal(err)
			}
		}
	}
	sign(root)
	rootBytes, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	hexDigest := hex.EncodeToString(digest[:])
	customFields := map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": platform, "architecture": runtime.GOARCH, "environment": "h3-test", "network": "new-release-test", "release_identity": "new-release-fixture", "release_version": int64(1), "source_revision": "test-source", "build_input_commitment": "test-inputs", "build_identity": "test-build", "dependency_identity": "test-dependencies", "sbom_identity": "test-sbom", "attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "protocol_phase": "announced", "build_safety_no_new_work_after": ref.Add(30 * 24 * time.Hour), "build_safety_terminate_after": ref.Add(180 * 24 * time.Hour)}
	var attestations []map[string]string
	for _, id := range []string{"test-builder-one", "test-builder-two"} {
		attestations = append(attestations, map[string]string{"builder_identity": id, "build_identity": "test-build", "source_revision": "test-source", "build_input_commitment": "test-inputs", "target_sha256": hexDigest})
	}
	customFields["builder_attestations"] = attestations
	custom, err := json.Marshal(customFields)
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(custom)
	target := metadata.Targets(expires)
	targetPath := "ardents/" + platform + "/endpoint"
	target.Signed.Targets[targetPath] = &metadata.TargetFiles{Length: int64(len(binary)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: targetPath, Custom: &raw}
	sign(target)
	targetBytes, err := target.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetDigest := sha256.Sum256(targetBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetBytes)), Hashes: metadata.Hashes{"sha256": targetDigest[:]}}
	sign(snapshot)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	sign(timestamp)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("schema=ardents-closed-alpha-enrollment-v3\ncohort=release-test\nrelease=new-release-fixture\nplatform=%s\nenvironment=h3-test\nnetwork=new-release-test\ntarget_path=%s\nartifact=%s\ntrusted_root=1.root.json\ncontrol_catalog=catalog.ac1\ndisclosure_root=catalog.pub\ncontrol_release=release.ac1\ncontrol_network=network.ac1\ncontrol_compatibility=compatibility.ac1\ncontrol_release_root=release.pub\ncontrol_network_root=network.pub\ncontrol_compatibility_root=compatibility.pub\ncorpus_authority=corpus.pub\ncontrol_artifact=%s\n", platform, targetPath, program, control)
	files := map[string][]byte{"RELEASE": []byte(descriptor), program: binary, "1.root.json": rootBytes, "timestamp.json": timestampBytes, "1.snapshot.json": snapshotBytes, "1.targets.json": targetBytes}
	for _, name := range []string{"catalog.ac1", "catalog.pub", "release.ac1", "network.ac1", "compatibility.ac1", "release.pub", "network.pub", "compatibility.pub", "corpus.pub", control} {
		files[name] = []byte("independent static companion " + name)
	}
	directory := filepath.Join(t.TempDir(), "bundle")
	if err = os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		data := files[name]
		digest := sha256.Sum256(data)
		fmt.Fprintf(&manifest, "%x  %s\n", digest, name)
		if err = os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(manifest.String()), 0600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256([]byte(manifest.String()))
	programPath := filepath.Join(directory, program)
	if err = os.Chmod(programPath, 0700); err != nil {
		t.Fatal(err)
	}
	return directory, fmt.Sprintf("%x", pin), programPath
}

func TestReleaseCompiledInitialVerificationAndRetainedRetry(t *testing.T) {
	directory, pin, program := signedConsumerBundle(t)
	history := filepath.Join(t.TempDir(), "release-history")
	for _, wanted := range []string{"release-accepted", "no-update"} {
		cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, pin, history, "2030-01-02T03:04:05Z")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("compiled consumer refused %v: %s", err, out)
		}
		var result struct {
			Outcome     string `json:"outcome"`
			RootVersion int64  `json:"root_version"`
		}
		if err = json.Unmarshal(out, &result); err != nil || result.Outcome != wanted || result.RootVersion != 1 {
			t.Fatalf("actual consumer result: %s", out)
		}
	}
	cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, strings.Repeat("0", 64), history, "2030-01-02T03:04:05Z")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "enrollment-refused") {
		t.Fatalf("wrong first pin accepted: %s", out)
	}
}

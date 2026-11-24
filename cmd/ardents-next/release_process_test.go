package main

import (
	"bytes"
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

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/release/endpoint"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// The fixture signs actual compiled consumer bytes with ephemeral test keys.
// Its inventory and custom fields are encoded independently of product codecs.
func signedConsumerProfile(t *testing.T, protected bool) (string, string, string) {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if protected {
		platform = "linux-amd64"
	}
	return signedConsumerPlatform(t, platform, protected)
}

func signedConsumerPlatform(t *testing.T, platform string, protected bool) (string, string, string) {
	return signedConsumerTargets(t, platform, protected, false, 1)
}

func signedConsumerTargets(t *testing.T, platform string, protected, generation bool, generationVersion int64) (string, string, string) {
	return signedConsumerResources(t, platform, protected, generation, generationVersion, nil)
}

func signedConsumerResources(t *testing.T, platform string, protected, generation bool, generationVersion int64, resources map[string][]byte) (string, string, string) {
	return signedConsumerRelease(t, platform, protected, generation, generationVersion, resources, nil, 1, 1)
}

func signedConsumerRelease(t *testing.T, platform string, protected, generation bool, generationVersion int64, resources map[string][]byte, authority *nodefixture.ReleaseAuthority, metadataVersion, releaseVersion int64, legacyCustody ...[]byte) (string, string, string) {
	t.Helper()
	_, architecture, validPlatform := strings.Cut(platform, "-")
	if !validPlatform || architecture == "" {
		t.Fatal("fixture requires an explicit target architecture")
	}
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	program := "ardents-" + platform
	if supplied, ok := resources[program]; protected && ok {
		binary = supplied
	}
	if strings.HasPrefix(platform, "windows-") {
		program += ".exe"
	}
	ref := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	expires := ref.Add(365 * 24 * time.Hour)
	if authority == nil {
		authority = nodefixture.NewReleaseAuthority(t, expires)
	}
	sign := func(value interface {
		Sign(signature.Signer) (*metadata.Signature, error)
	}) {
		authority.Sign(t, value)
	}
	rootBytes := authority.RootBytes()
	protectedFiles := map[string][]byte{program: binary}
	if protected {
		// Exact ADR-0119 names, independently spelled rather than projected
		// from the consumer's exclusion list.
		names := []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"}
		digests := make(map[string]string)
		for _, name := range names {
			if name != program {
				if supplied, ok := resources[name]; ok {
					protectedFiles[name] = supplied
				} else {
					protectedFiles[name] = []byte("independent protected resource " + name)
				}
			}
			digest := sha256.Sum256(protectedFiles[name])
			digests[name] = hex.EncodeToString(digest[:])
		}
		encoded, err := json.Marshal(digests)
		if err != nil {
			t.Fatal(err)
		}
		protectedFiles["protected-endpoint.json"] = []byte(fmt.Sprintf("{\"schema\":\"ardents-protected-endpoint-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"new-release-fixture\",\"release_version\":%d,\"files\":%s}\n", releaseVersion, encoded))
	}
	digest := sha256.Sum256(binary)
	hexDigest := hex.EncodeToString(digest[:])
	customFields := map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": platform, "architecture": architecture, "environment": "h3-test", "network": "new-release-test", "release_identity": "new-release-fixture", "release_version": int64(1), "source_revision": "test-source", "build_input_commitment": "test-inputs", "build_identity": "test-build", "dependency_identity": "test-dependencies", "sbom_identity": "test-sbom", "attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "protocol_phase": "announced", "build_safety_no_new_work_after": ref.Add(30 * 24 * time.Hour), "build_safety_terminate_after": ref.Add(180 * 24 * time.Hour)}
	customFields["release_version"] = releaseVersion
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
	target.Signed.Version = metadataVersion
	targetPath := "ardents/" + platform + "/endpoint"
	target.Signed.Targets[targetPath] = &metadata.TargetFiles{Length: int64(len(binary)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: targetPath, Custom: &raw}
	if generation {
		generationPath := "ardents/linux-amd64/protected-endpoint"
		generationBytes := protectedFiles["protected-endpoint.json"]
		generationDigest := sha256.Sum256(generationBytes)
		generationFields := make(map[string]any, len(customFields))
		for name, value := range customFields {
			generationFields[name] = value
		}
		generationFields["release_version"] = generationVersion
		var generationAttestations []map[string]string
		for _, id := range []string{"test-builder-one", "test-builder-two"} {
			generationAttestations = append(generationAttestations, map[string]string{"builder_identity": id, "build_identity": "test-build", "source_revision": "test-source", "build_input_commitment": "test-inputs", "target_sha256": hex.EncodeToString(generationDigest[:])})
		}
		generationFields["builder_attestations"] = generationAttestations
		encoded, err := json.Marshal(generationFields)
		if err != nil {
			t.Fatal(err)
		}
		generationRaw := json.RawMessage(encoded)
		target.Signed.Targets[generationPath] = &metadata.TargetFiles{Length: int64(len(generationBytes)), Hashes: metadata.Hashes{"sha256": generationDigest[:]}, Path: generationPath, Custom: &generationRaw}
	}
	sign(target)
	targetBytes, err := target.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetDigest := sha256.Sum256(targetBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Version = metadataVersion
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: metadataVersion, Length: int64(len(targetBytes)), Hashes: metadata.Hashes{"sha256": targetDigest[:]}}
	sign(snapshot)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Version = metadataVersion
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: metadataVersion, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	sign(timestamp)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("schema=ardents-closed-alpha-enrollment-v6\ncohort=release-test\nrelease=new-release-fixture\nplatform=%s\nenvironment=h3-test\nnetwork=new-release-test\ntarget_path=%s\nartifact=%s\ntrusted_root=1.root.json\n", platform, targetPath, program)
	files := map[string][]byte{"RELEASE": []byte(descriptor), program: binary,
		"1.root.json": rootBytes, "timestamp.json": timestampBytes, fmt.Sprintf("%d.snapshot.json", metadataVersion): snapshotBytes, fmt.Sprintf("%d.targets.json", metadataVersion): targetBytes}
	for name, data := range protectedFiles {
		files[name] = data
	}
	if len(legacyCustody) != 0 {
		files["RELEASE"] = bytes.Replace(files["RELEASE"], []byte("enrollment-v6"), []byte("enrollment-v5"), 1)
		files["ardents-custody-"+platform] = legacyCustody[0]
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

func TestReleaseInitialProtectedInventoryUsesOnlyMetadata(t *testing.T) {
	directory, pin, program := signedConsumerProfile(t, true)
	bundle, err := enrollment.Verify(t.Context(), enrollment.Request{BundleRoot: directory, ExecutablePath: program, ManifestSHA256: pin})
	if err != nil {
		t.Fatal(err)
	}
	facts, ok := bundle.Facts()
	if !ok || !facts.Protected || len(bundle.Names()) != 15 {
		t.Fatal("fixture lacks genuinely verified protected inventory")
	}
	input, ok := generationauthorization.InitialInputs(bundle, time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC))
	// This metadata-only fixture models the protected linux-amd64 artifact,
	// not execution of that artifact on the current developer host.
	input.Local.Architecture = "amd64"
	if !ok {
		t.Fatal("verified inventory projection refused")
	}
	verifier, err := release.Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, expected := range []release.Outcome{release.OutcomeReleaseAccepted, release.OutcomeNoUpdate} {
		decision := verifier.Evaluate(t.Context(), input)
		if _, ok := decision.Authorization(); !ok || decision.Outcome != expected {
			t.Fatalf("genuine protected bundle refused: %s: %v", decision.Outcome, decision.Err())
		}
	}
	if len(input.Files) != 3 {
		t.Fatalf("static inventory escaped into metadata: got %d metadata files, want 3", len(input.Files))
	}
	for _, name := range []string{"timestamp.json", "1.snapshot.json", "1.targets.json"} {
		if _, ok := input.Files["https://release.invalid/metadata/"+name]; !ok {
			t.Fatalf("missing actual metadata %s", name)
		}
	}
}

func TestReleaseCompiledForeignPlatformRefusesBeforeHistory(t *testing.T) {
	foreign := "windows-amd64"
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		// The host must be able to execute the fixture before it can refuse
		// the enrolled platform. Change the declared CPU but keep .exe.
		foreign = "windows-arm64"
	}
	directory, pin, program := signedConsumerPlatform(t, foreign, false)
	history := filepath.Join(t.TempDir(), "absent-history")
	cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, pin, history, "2030-01-02T03:04:05Z")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "release-incompatible") {
		t.Fatalf("signed foreign platform was not refused: %v: %s", err, out)
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatalf("foreign platform opened history: %v", err)
	}
}

func TestReleaseCompiledInitialVerificationAndRetainedRetry(t *testing.T) {
	t.Run("general", func(t *testing.T) { verifyCompiledReleaseProfile(t, false) })
	// Actual protected executable invocation is selected only where the
	// current linux-amd64 artifact contract applies. Its portable projection
	// is independently exercised on every native test platform above.
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		t.Run("protected", func(t *testing.T) { verifyCompiledReleaseProfile(t, true) })
	}
}

func verifyCompiledReleaseProfile(t *testing.T, protected bool) {
	t.Helper()
	directory, pin, program := signedConsumerProfile(t, protected)
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

package endpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// SignedRelease independently encodes and signs the fixture inventory for the
// supplied command. The resulting bundle can drive Enrollment and Release.
func SignedRelease(t *testing.T, command string, platform string, protected, generation bool, generationVersion int64, resources map[string][]byte, authority *nodefixture.ReleaseAuthority, metadataVersion, releaseVersion int64, legacyCustody ...[]byte) (string, string, string) {
	t.Helper()
	_, architecture, validPlatform := strings.Cut(platform, "-")
	if !validPlatform || architecture == "" {
		t.Fatal("fixture requires an explicit target architecture")
	}
	binary, err := os.ReadFile(command)
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

// FixtureDigest spells the SHA-256 used by the independent fixture declarations.
func FixtureDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

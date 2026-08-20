//go:build ignore

package main

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestBoundedFetcherEnvelope(t *testing.T) {
	files := map[string][]byte{}
	for index := 0; index < 9; index++ {
		files[fmt.Sprintf("https://release.invalid/metadata/%d.targets.json", index)] = validMetadataBlob(int(profileMetadataFileBytes))
	}
	fetch, err := newBoundedFetcher("https://release.invalid/metadata/", files)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 8; index++ {
		data, err := fetch.DownloadFile(fmt.Sprintf("https://release.invalid/metadata/%d.targets.json", index), profileMetadataFileBytes, 0)
		if err != nil || len(data) != int(profileMetadataFileBytes) {
			t.Fatalf("fetch %d: len=%d err=%v", index, len(data), err)
		}
	}
	_, err = fetch.DownloadFile("https://release.invalid/metadata/8.targets.json", profileMetadataFileBytes, 0)
	if !errors.Is(err, errProfileAggregate) {
		t.Fatalf("aggregate limit: %v", err)
	}
}

func TestBoundedFetcherRejectsCountAndURL(t *testing.T) {
	files := map[string][]byte{"https://release.invalid/metadata/timestamp.json": validMetadataBlob(128)}
	fetch, err := newBoundedFetcher("https://release.invalid/metadata/", files)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < profileFetches; index++ {
		_, err = fetch.DownloadFile("https://release.invalid/metadata/timestamp.json", 128, 0)
		if err != nil {
			t.Fatalf("fetch %d: %v", index, err)
		}
	}
	_, err = fetch.DownloadFile("https://release.invalid/metadata/timestamp.json", 128, 0)
	if !errors.Is(err, errProfileCount) {
		t.Fatalf("fetch count: %v", err)
	}

	for _, raw := range []string{
		"http://release.invalid/metadata/timestamp.json",
		"https://other.invalid/metadata/timestamp.json",
		"https://release.invalid/metadata/../secret",
		"https://release.invalid/metadata/%2e%2e/secret",
		"https://release.invalid/metadata/%5c..%5csecret",
		"https://release.invalid/metadata/timestamp.json?retry=1",
	} {
		other, newErr := newBoundedFetcher("https://release.invalid/metadata/", files)
		if newErr != nil {
			t.Fatal(newErr)
		}
		if _, newErr = other.DownloadFile(raw, 1, 0); !errors.Is(newErr, errProfileURL) {
			t.Fatalf("URL %q: %v", raw, newErr)
		}
	}
}

func TestProfileConfigDisablesCacheAndDelegation(t *testing.T) {
	fetch, err := newBoundedFetcher("https://release.invalid/metadata/", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := newProfileConfig([]byte(`{"signed":{"_type":"root","keys":{},"roles":{}},"signatures":[]}`), fetch)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DisableLocalCache || cfg.UnsafeLocalMode || cfg.MaxDelegations != 0 || cfg.MaxRootRotations != profileRootRotations {
		t.Fatalf("unsafe profile: %+v", cfg)
	}
	if cfg.RootMaxLength != profileMetadataFileBytes || cfg.TimestampMaxLength != profileMetadataFileBytes || cfg.SnapshotMaxLength != profileMetadataFileBytes || cfg.TargetsMaxLength != profileMetadataFileBytes {
		t.Fatalf("metadata limits are not frozen: %+v", cfg)
	}
}

func TestProfileEnvelope(t *testing.T) {
	set, target, artifact := maximumProfileEnvelope()
	started := time.Now()
	if err := validateProfileShape(set); err != nil {
		t.Fatal(err)
	}
	if err := verifyArtifact(target, artifact); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("profile decision took %s", elapsed)
	}
}

func TestVerifiedProfileEnvelope(t *testing.T) {
	repository := maximumSignedRepository(t)
	started := time.Now()
	if err := verifyRepositoryDecision(repository.root, repository.files, repository.targetPath, repository.artifact); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("verified profile decision took %s", elapsed)
	}
	if status, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(line, "VmHWM:") {
				t.Log(line)
				break
			}
		}
	}
}

func TestProfileRejectsExcessAndDelegation(t *testing.T) {
	set, _, _ := maximumProfileEnvelope()
	top := set.Targets[metadata.TARGETS]
	top.Signatures = append(top.Signatures, metadata.Signature{})
	if !errors.Is(validateProfileShape(set), errProfileCount) {
		t.Fatal("signature excess was not rejected")
	}
	top.Signatures = top.Signatures[:profileSignatures]
	top.Signed.Delegations = &metadata.Delegations{}
	if err := validateProfileShape(set); err == nil || err.Error() != "delegated targets are disabled" {
		t.Fatalf("delegation: %v", err)
	}

	root := metadata.Root(time.Now().UTC().Add(24 * time.Hour))
	root.Signatures = syntheticSignatures(profileSignatures + 1)
	encodedRoot, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(validateMetadataEnvelope(encodedRoot), errProfileCount) {
		t.Fatal("preflight accepted excess signatures")
	}

	targets := metadata.Targets(time.Now().UTC().Add(24 * time.Hour))
	targets.Signed.Delegations = &metadata.Delegations{}
	encodedTargets, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateMetadataEnvelope(encodedTargets); err == nil || err.Error() != "delegated targets are disabled" {
		t.Fatalf("preflight delegation: %v", err)
	}
}

func BenchmarkProfileEnvelope(b *testing.B) {
	set, target, artifact := maximumProfileEnvelope()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := validateProfileShape(set); err != nil {
			b.Fatal(err)
		}
		if err := verifyArtifact(target, artifact); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifiedProfileEnvelope(b *testing.B) {
	repository := maximumSignedRepository(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if err := verifyRepositoryDecision(repository.root, repository.files, repository.targetPath, repository.artifact); err != nil {
			b.Fatal(err)
		}
	}
}

type syntheticRepository struct {
	root       []byte
	files      map[string][]byte
	targetPath string
	artifact   []byte
}

type testFataler interface {
	Helper()
	Fatal(...any)
}

type signable interface {
	Sign(signature.Signer) (*metadata.Signature, error)
}

func maximumSignedRepository(t testFataler) syntheticRepository {
	t.Helper()
	expires := time.Now().UTC().Add(24 * time.Hour)
	root := metadata.Root(expires)
	signers := make([]signature.Signer, 0, profileKeys)
	roleKeyIDs := make([]string, 0, 5)
	for index := 0; index < profileKeys; index++ {
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
		signers = append(signers, signer)
		if len(roleKeyIDs) < 5 {
			roleKeyIDs = append(roleKeyIDs, id)
		}
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), roleKeyIDs...), Threshold: 3}
	}
	signAll(t, root, signers)
	rootBytes, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}

	artifact := make([]byte, profileMetadataFileBytes)
	artifactHash := sha256.Sum256(artifact)
	targetPath := "ardents/windows-amd64/application"
	targets := metadata.Targets(expires)
	for index := 0; index < profileTargets; index++ {
		path := fmt.Sprintf("release/target-%04d", index)
		targets.Signed.Targets[path] = &metadata.TargetFiles{Length: 1, Hashes: metadata.Hashes{"sha256": make([]byte, sha256.Size)}, Path: path}
	}
	targets.Signed.Targets[targetPath] = &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": artifactHash[:]}, Path: targetPath}
	delete(targets.Signed.Targets, "release/target-0000")
	signAll(t, targets, signers)
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}

	targetsHash := sha256.Sum256(targetsBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsHash[:]}}
	signAll(t, snapshot, signers)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}

	snapshotHash := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotHash[:]}}
	signAll(t, timestamp, signers)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}

	return syntheticRepository{
		root: rootBytes,
		files: map[string][]byte{
			"https://release.invalid/metadata/timestamp.json":  timestampBytes,
			"https://release.invalid/metadata/1.snapshot.json": snapshotBytes,
			"https://release.invalid/metadata/1.targets.json":  targetsBytes,
		},
		targetPath: targetPath,
		artifact:   artifact,
	}
}

func signAll(t testFataler, metadata signable, signers []signature.Signer) {
	t.Helper()
	for _, signer := range signers {
		if _, err := metadata.Sign(signer); err != nil {
			t.Fatal(err)
		}
	}
}

func validMetadataBlob(size int) []byte {
	prefix := []byte(`{"signed":{"_type":"timestamp","meta":{},"padding":"`)
	suffix := []byte(`"},"signatures":[]}`)
	if size < len(prefix)+len(suffix) {
		panic("metadata blob is too small")
	}
	data := make([]byte, 0, size)
	data = append(data, prefix...)
	data = append(data, strings.Repeat("x", size-len(prefix)-len(suffix))...)
	data = append(data, suffix...)
	return data
}

//go:build linux

package endpoint_test

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// These fixtures build the ephemeral TUF-compatible Release metadata and
// canonical manifest for one exact enrolled artifact. The per-user Portable
// lifecycle that formerly consumed the assembled bundle is retired (#617
// S3.5); the surviving alpha-control inspection regressions still reuse this
// metadata to pin an exact artifact and Release decision offline. The bundle,
// keys and metadata are test-only and ephemeral.

type enrolledRuntimeKey struct {
	public ed25519.PublicKey
	signer signature.Signer
}

func enrolledRuntimeMetadata(t *testing.T, artifact []byte, targetPath, platform string, now time.Time) (map[string][]byte, []byte) {
	// The alpha-control fixture selects a completed required-protocol overlap so
	// its first fresh alpha-control observation is exactly release-accepted.
	metadataFiles, rootBytes, _ := enrolledRuntimeMetadataWithKeysAt(t, artifact, targetPath, platform, now, now.Add(-100*24*time.Hour))
	return metadataFiles, rootBytes
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

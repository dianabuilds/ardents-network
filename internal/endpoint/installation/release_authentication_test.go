package installation

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestProtectedTargetsUseFreshProofsFromOneSignedSet(t *testing.T) {
	enrolled := protectedReleaseFixture(t, nil, false)
	verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := verifier.Close(); err != nil {
			t.Error(err)
		}
	})
	authorization, err := Authenticate(context.Background(), verifier, enrolled)
	if err != nil {
		t.Fatal(err)
	}
	programProof, generationProof := authorization.Targets()
	program, programOK := programProof.AcceptedDecision()
	generation, generationOK := generationProof.AcceptedDecision()
	if !programOK || !generationOK || program.Outcome != release.OutcomeReleaseAccepted ||
		generation.Outcome != release.OutcomeNoUpdate || program.Path != programTarget || generation.Path != generationTarget ||
		bytes.Equal(program.Digest, generation.Digest) || !bytes.Equal(program.Floors.TargetsDigest, generation.Floors.TargetsDigest) {
		t.Fatalf("proofs do not cover two distinct targets in one set: program=%+v generation=%+v", program, generation)
	}
	// A fresh repeated transaction must still produce two opaque proofs after
	// the shared floor has committed; display outcomes alone are insufficient.
	repeated, err := Authenticate(context.Background(), verifier, enrolled)
	if err != nil {
		t.Fatal(err)
	}
	first, second := repeated.Targets()
	for _, proof := range []release.Authorization{first, second} {
		decision, ok := proof.AcceptedDecision()
		if !ok || decision.Outcome != release.OutcomeNoUpdate {
			t.Fatalf("repeat lacks fresh no-update proof: %+v", decision)
		}
	}
	for _, proof := range []release.Authorization{func() release.Authorization { p, _ := (Authorization{}).Targets(); return p }(), func() release.Authorization { _, g := (Authorization{}).Targets(); return g }()} {
		if _, ok := proof.AcceptedDecision(); ok {
			t.Fatal("zero installation proof authorizes a target")
		}
	}
}

func TestProtectedTargetsRefuseMissingGenerationProof(t *testing.T) {
	enrolled := protectedReleaseFixture(t, nil, true)
	verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	if _, err := Authenticate(context.Background(), verifier, enrolled); err == nil {
		t.Fatal("executable-only signed set produced a protected generation proof")
	} else {
		t.Logf("missing generation target refusal: %v", err)
	}
	decision := verifier.Evaluate(context.Background(), enrolled.Inputs)
	if _, ok := decision.Authorization(); !ok || decision.Outcome != release.OutcomeNoUpdate {
		t.Fatalf("generation refusal must retain the executable's committed floors: %+v", decision)
	}
}

func TestProtectedTargetsRefuseAuthenticIdentityMismatch(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
		value any
	}{
		{"release-identity", "release_identity", "other-release"},
		{"release-version", "release_version", int64(2)},
		{"network", "network", "other-network"},
	} {
		t.Run(test.name, func(t *testing.T) {
			enrolled := protectedReleaseFixture(t, func(custom map[string]any) { custom[test.field] = test.value }, false)
			verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
			if err != nil {
				t.Fatal(err)
			}
			defer verifier.Close()
			if authorization, err := Authenticate(context.Background(), verifier, enrolled); err == nil {
				t.Fatalf("authentic conflicting generation crossed protection boundary: %+v", authorization)
			}
		})
	}
}

func TestProtectedTargetsRefuseSubstitutionBeforeReleaseFloors(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*enrollment.Verified)
	}{
		{"worker-substitution", func(e *enrollment.Verified) { e.ProtectedFiles["ardents-text-linux-amd64"] = []byte("substitute") }},
		{"partial-inventory", func(e *enrollment.Verified) { delete(e.ProtectedFiles, "ardents-text.conf") }},
		{"different-program", func(e *enrollment.Verified) { e.Inputs.Artifact = []byte("other-program") }},
		{"missing-descriptor", func(e *enrollment.Verified) { e.ProtectedDescriptor = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			enrolled := protectedReleaseFixture(t, nil, false)
			original := enrolled.Inputs
			test.change(&enrolled)
			verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
			if err != nil {
				t.Fatal(err)
			}
			defer verifier.Close()
			if _, err := Authenticate(context.Background(), verifier, enrolled); err == nil {
				t.Fatal("invalid generation accepted")
			}
			decision := verifier.Evaluate(context.Background(), original)
			if decision.Outcome != release.OutcomeReleaseAccepted {
				t.Fatalf("preflight refusal mutated Release floors: %+v", decision)
			}
		})
	}
}

// protectedReleaseFixture signs synthetic public metadata solely for behavior
// tests. Its builder identities and qualification field are not release evidence.
func protectedReleaseFixture(t *testing.T, changeGeneration func(map[string]any), omitGeneration bool) enrollment.Verified {
	t.Helper()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	resources := map[string][]byte{}
	digests := map[string]string{}
	for _, name := range []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"} {
		resources[name] = []byte("fixture resource: " + name)
		if name == "ardents-endpoint.service" {
			var err error
			resources[name], err = os.ReadFile("../../../packaging/text-worker/ardents-endpoint.service")
			if err != nil {
				t.Fatal(err)
			}
		}
		digest := sha256.Sum256(resources[name])
		digests[name] = hex.EncodeToString(digest[:])
	}
	descriptor, err := json.Marshal(struct {
		Schema          string            `json:"schema"`
		Platform        string            `json:"platform"`
		ReleaseIdentity string            `json:"release_identity"`
		ReleaseVersion  int64             `json:"release_version"`
		Files           map[string]string `json:"files"`
	}{"ardents-protected-endpoint-artifact-v1", "linux-amd64", "fixture-release", 1, digests})
	if err != nil {
		t.Fatal(err)
	}
	descriptor = append(descriptor, '\n')
	root := metadata.Root(now.Add(time.Hour))
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": "alpha", "ardents_network": "alpha-network-1"}
	var signers []signature.Signer
	var keyIDs []string
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
		keyIDs = append(keyIDs, id)
		signers = append(signers, signer)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: keyIDs, Threshold: 3}
	}
	sign := func(value interface {
		Sign(signature.Signer) (*metadata.Signature, error)
	}) {
		t.Helper()
		for _, signer := range signers {
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
	targets := metadata.Targets(now.Add(time.Hour))
	for _, target := range []struct {
		path     string
		artifact []byte
	}{{programTarget, resources["ardents-linux-amd64"]}, {generationTarget, descriptor}} {
		if target.path == generationTarget && omitGeneration {
			continue
		}
		digest := sha256.Sum256(target.artifact)
		custom := map[string]any{
			"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": "linux-amd64", "architecture": "amd64",
			"environment": "alpha", "network": "alpha-network-1", "release_identity": "fixture-release", "release_version": 1,
			"source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "build_identity": "fixture-build",
			"dependency_identity": "fixture-dependencies", "sbom_identity": "fixture-sbom", "attestation_policy": "two-builder",
			"qualification": "qualified", "build_state": "current", "protocol_phase": "required", "protocol_overlapped_since": now.Add(-100 * 24 * time.Hour),
			"capacity_ready": true, "drain_ready": true, "build_safety_no_new_work_after": now.Add(20 * time.Minute), "build_safety_terminate_after": now.Add(40 * time.Minute),
			"builder_attestations": []map[string]string{
				{"builder_identity": "builder-a", "build_identity": "fixture-build", "source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "target_sha256": hex.EncodeToString(digest[:])},
				{"builder_identity": "builder-b", "build_identity": "fixture-build", "source_revision": "fixture-source", "build_input_commitment": "fixture-inputs", "target_sha256": hex.EncodeToString(digest[:])},
			},
		}
		if target.path == generationTarget && changeGeneration != nil {
			changeGeneration(custom)
		}
		raw, err := json.Marshal(custom)
		if err != nil {
			t.Fatal(err)
		}
		targets.Signed.Targets[target.path] = &metadata.TargetFiles{Length: int64(len(target.artifact)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: target.path, Custom: (*json.RawMessage)(&raw)}
	}
	sign(targets)
	targetBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetDigest := sha256.Sum256(targetBytes)
	snapshot := metadata.Snapshot(now.Add(time.Hour))
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetBytes)), Hashes: metadata.Hashes{"sha256": targetDigest[:]}}
	sign(snapshot)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(now.Add(time.Hour))
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	sign(timestamp)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return enrollment.Verified{ProtectedDescriptor: descriptor, ProtectedFiles: resources, Inputs: release.Inputs{
		RootBytes: rootBytes, Files: map[string][]byte{release.MetadataURL("timestamp.json"): timestampBytes, release.MetadataURL("1.snapshot.json"): snapshotBytes, release.MetadataURL("1.targets.json"): targetBytes},
		Artifact: resources["ardents-linux-amd64"], TargetPath: programTarget, Local: release.LocalEnvironment{Environment: "alpha", Network: "alpha-network-1", Platform: "linux-amd64", Architecture: "amd64", RefTime: now},
	}}
}

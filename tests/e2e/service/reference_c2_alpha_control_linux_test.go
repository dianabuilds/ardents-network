//go:build linux

package service_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

type referenceC2AlphaControlFixture struct {
	enrollment, artifact, control, controlRoot string
	disclosurePrivate                          ed25519.PrivateKey
	corpusPublic                               ed25519.PublicKey
	network                                    [32]byte
	now                                        time.Time
}

// stageReferenceC2AlphaCorpus uses the same separately compiled participant
// command as alpha-control qualification. It writes the exact persistent floor
// before the C2 User starts, so the fixture's only alpha resolution input is a
// real accepted-command result.
func stageReferenceC2AlphaCorpus(t *testing.T, endpoint, control string, publicationPath string, authority ed25519.PublicKey, private ed25519.PrivateKey, floorRoot string, network [32]byte, linkText string) {
	t.Helper()
	if endpoint == "" || control == "" {
		t.Fatal("Linux C2 alpha-control commands are unavailable")
	}
	fixture := referenceC2AlphaControlFixtureFor(t, endpoint, control, authority, private, network)
	publication := referenceC2AlphaPublication(t, publicationPath, authority, linkText)
	catalog := referenceC2AlphaCatalog(t, fixture, publication)
	directory := t.TempDir()
	catalogPath, corpusPath := filepath.Join(directory, "catalog.ac2"), filepath.Join(directory, "corpus.anc")
	referenceC2AlphaWrite(t, catalogPath, catalog, 0o600)
	referenceC2AlphaWrite(t, corpusPath, publication.corpus, 0o600)
	arguments := []string{"accept-alpha-corpus", "--enrollment", fixture.enrollment, "--artifact", fixture.artifact,
		"--control-state-root", fixture.controlRoot, "--corpus-state-root", floorRoot, "--catalog", catalogPath, "--corpus", corpusPath,
		"--at", fixture.now.Format(time.RFC3339)}
	output, err := exec.Command(fixture.control, arguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("accept alpha corpus before C2: %v\n%s", err, output)
	}
	var report struct {
		Schema, Corpus, Network string
		Serial                  uint64
	}
	if err := json.Unmarshal(output, &report); err != nil || report.Schema != "ardents-alpha-corpus-acceptance-v1" ||
		report.Corpus != "accepted" || report.Network != hex.EncodeToString(network[:]) || report.Serial != 1 {
		t.Fatalf("accept alpha corpus report = %s / %+v / %v", output, report, err)
	}
}

type referenceC2AlphaPublicationValue struct {
	corpus   []byte
	notAfter time.Time
}

func referenceC2AlphaPublication(t *testing.T, path string, authority ed25519.PublicKey, linkText string) referenceC2AlphaPublicationValue {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var publication struct{ AlphaAuthorityPublic, AlphaCorpus, AlphaLink string }
	if err := json.Unmarshal(raw, &publication); err != nil || publication.AlphaAuthorityPublic != hex.EncodeToString(authority) || publication.AlphaLink != linkText {
		t.Fatal("C2 Publisher alpha publication is invalid")
	}
	corpus, err := base64.RawStdEncoding.DecodeString(publication.AlphaCorpus)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := alpha.OpenCorpus(authority, corpus)
	if err != nil {
		t.Fatal("C2 Publisher alpha corpus is invalid")
	}
	return referenceC2AlphaPublicationValue{corpus: corpus, notAfter: opened.NotAfter()}
}

func referenceC2AlphaCatalog(t *testing.T, fixture referenceC2AlphaControlFixture, publication referenceC2AlphaPublicationValue) []byte {
	t.Helper()
	catalog := alphacontrol.CatalogV2{Cohort: "reference-c2", Generation: 1, NotBefore: fixture.now.Add(-time.Minute), NotAfter: fixture.now.Add(20 * time.Minute)}
	for index := 0; index < 3; index++ {
		body := []byte{byte(index + 1)}
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: [32]byte{byte(index + 1)},
			Generation: 1, NotAfter: catalog.NotAfter, Size: uint32(len(body)), Digest: sha256.Sum256(body)}
	}
	catalog.Components[3] = alphacontrol.Component{Class: alphacontrol.ComponentCorpus, RootID: sha256.Sum256(fixture.corpusPublic),
		Generation: 1, NotAfter: publication.notAfter, Size: uint32(len(publication.corpus)), Digest: sha256.Sum256(publication.corpus)}
	raw, err := alphacontrol.SignV2(catalog, fixture.disclosurePrivate)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func referenceC2AlphaControlFixtureFor(t *testing.T, endpoint, control string, corpusPublic ed25519.PublicKey, corpusPrivate ed25519.PrivateKey, network [32]byte) referenceC2AlphaControlFixture {
	t.Helper()
	if len(corpusPublic) != ed25519.PublicKeySize || len(corpusPrivate) != ed25519.PrivateKeySize || !bytes.Equal(corpusPublic, corpusPrivate.Public().(ed25519.PublicKey)) {
		t.Fatal("C2 alpha corpus key pair is invalid")
	}
	artifact, err := os.ReadFile(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	controlArtifact, err := os.ReadFile(control)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	platform, targetPath := runtime.GOOS+"-"+runtime.GOARCH, "ardents/"+runtime.GOOS+"-"+runtime.GOARCH+"/endpoint"
	metadataFiles, rootBytes := referenceC2AlphaMetadata(t, artifact, targetPath, platform, now)
	decision := referenceC2AlphaReleaseDecision(t, rootBytes, metadataFiles, artifact, targetPath, platform, now)
	components, roots, disclosurePrivate := referenceC2AlphaStatements(t, decision, network, now)
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	artifactName, controlName := "ardents-"+platform, "ardents-control-"+platform
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v3", "cohort=reference-c2", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json",
		"control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1", "control_compatibility=compatibility.ac1",
		"control_release_root=release.pub", "control_network_root=network.pub", "control_compatibility_root=compatibility.pub", "corpus_authority=corpus.pub", "control_artifact=" + controlName,
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact, controlName: controlArtifact, "catalog.ac1": referenceC2AlphaCatalogV1(t, components, roots, disclosurePrivate, now),
		"catalog.pub": disclosurePrivate.Public().(ed25519.PublicKey), "release.ac1": components[0], "network.ac1": components[1], "compatibility.ac1": components[2],
		"release.pub": roots[0], "network.pub": roots[1], "compatibility.pub": roots[2], "corpus.pub": corpusPublic}
	for name, value := range metadataFiles {
		files[name] = value
	}
	names := []string{"1.root.json", "1.snapshot.json", "1.targets.json", "RELEASE", controlName, artifactName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "corpus.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"}
	for _, name := range names {
		mode := os.FileMode(0o600)
		if name == artifactName || name == controlName {
			mode = 0o700
		}
		referenceC2AlphaWrite(t, filepath.Join(bundle, name), files[name], mode)
	}
	manifest := referenceC2AlphaManifest(t, names, files)
	referenceC2AlphaWrite(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	pin := sha256.Sum256(manifest)
	enrollment := filepath.Join(t.TempDir(), "alpha-enrollment.json")
	input, err := json.Marshal(map[string]string{"schema": "ardents-alpha-enrollment-input-v1", "bundle_root": bundle, "cohort": "reference-c2", "release": "alpha-1",
		"platform": platform, "manifest_sha256": hex.EncodeToString(pin[:]), "environment": "alpha", "network": "alpha-network-1", "target_path": targetPath})
	if err != nil {
		t.Fatal(err)
	}
	referenceC2AlphaWrite(t, enrollment, input, 0o600)
	return referenceC2AlphaControlFixture{enrollment: enrollment, artifact: filepath.Join(bundle, artifactName), control: filepath.Join(bundle, controlName), controlRoot: filepath.Join(t.TempDir(), "control"),
		disclosurePrivate: disclosurePrivate, corpusPublic: corpusPublic, network: network, now: now}
}

func referenceC2AlphaReleaseDecision(t *testing.T, root []byte, metadataFiles map[string][]byte, artifact []byte, targetPath, platform string, now time.Time) release.Decision {
	t.Helper()
	verifier, err := release.Open(filepath.Join(t.TempDir(), "release"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = verifier.Close() })
	decision := verifier.Evaluate(context.Background(), release.Inputs{RootBytes: root, Files: map[string][]byte{
		release.MetadataURL("timestamp.json"): metadataFiles["timestamp.json"], release.MetadataURL("1.snapshot.json"): metadataFiles["1.snapshot.json"], release.MetadataURL("1.targets.json"): metadataFiles["1.targets.json"]},
		TargetPath: targetPath, Artifact: artifact, Local: release.LocalEnvironment{Environment: "alpha", Network: "alpha-network-1", Platform: platform, Architecture: runtime.GOARCH, RefTime: now}})
	if decision.Outcome != release.OutcomeReleaseAccepted && decision.Outcome != release.OutcomeNoUpdate {
		t.Fatalf("C2 alpha release decision = %+v", decision)
	}
	return decision
}

func referenceC2AlphaStatements(t *testing.T, decision release.Decision, network [32]byte, now time.Time) ([3][]byte, [3][]byte, ed25519.PrivateKey) {
	t.Helper()
	_, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var artifactDigest [32]byte
	copy(artifactDigest[:], decision.Digest)
	releaseBody, err := inspection.EncodeReleaseEvidence(inspection.ReleaseEvidence{ArtifactDigest: artifactDigest, TargetPath: decision.Path,
		ReleaseIdentity: decision.ReleaseIdentity, BuildIdentity: decision.BuildIdentity, ProtocolPhase: decision.ProtocolPhase, BuildState: decision.BuildState})
	if err != nil {
		t.Fatal(err)
	}
	authority, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	epoch, epochDigest := referenceC2AlphaEpoch(network, now, authorityPrivate)
	networkBody, err := inspection.EncodeNetworkEvidence(inspection.NetworkEvidence{NetworkID: network, EpochDigest: epochDigest, Profile: "h3-role-probe-v1", Threshold: 1, Authorities: []ed25519.PublicKey{authority}, Epoch: epoch})
	if err != nil {
		t.Fatal(err)
	}
	compatibilityBody, err := inspection.EncodeCompatibilityEvidence(inspection.CompatibilityEvidence{ReleaseDigest: artifactDigest,
		ReleaseBuildIdentity: decision.BuildIdentity, ProtocolPhase: decision.ProtocolPhase, NetworkDigest: epochDigest, NetworkEpoch: 1, NetworkProfile: "h3-role-probe-v1"})
	if err != nil {
		t.Fatal(err)
	}
	bodies := [3][]byte{releaseBody, networkBody, compatibilityBody}
	var components [3][]byte
	var roots [3][]byte
	for index, body := range bodies {
		public, private, keyErr := ed25519.GenerateKey(rand.Reader)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		components[index], keyErr = alphacontrol.SignComponent(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1), Generation: 1,
			NotBefore: now.Add(-time.Minute), NotAfter: now.Add(20 * time.Minute), Body: body}, private)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		roots[index] = public
	}
	return components, roots, disclosurePrivate
}

func referenceC2AlphaCatalogV1(t *testing.T, components [3][]byte, roots [3][]byte, disclosure ed25519.PrivateKey, now time.Time) []byte {
	t.Helper()
	catalog := alphacontrol.Catalog{Cohort: "reference-c2", Generation: 1, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(20 * time.Minute)}
	for index := range components {
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: sha256.Sum256(roots[index]), Generation: 1,
			NotAfter: catalog.NotAfter, Size: uint32(len(components[index])), Digest: sha256.Sum256(components[index])}
	}
	raw, err := alphacontrol.Sign(catalog, disclosure)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type referenceC2AlphaKey struct {
	public ed25519.PublicKey
	signer signature.Signer
}

func referenceC2AlphaMetadata(t *testing.T, artifact []byte, targetPath, platform string, now time.Time) (map[string][]byte, []byte) {
	t.Helper()
	keys := make([]referenceC2AlphaKey, 0, 5)
	for range 5 {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signature.LoadSigner(private, crypto.Hash(0))
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, referenceC2AlphaKey{public: public, signer: signer})
	}
	sign := func(value interface {
		Sign(signature.Signer) (*metadata.Signature, error)
	}, count int) {
		for _, key := range keys[:count] {
			if _, err := value.Sign(key.signer); err != nil {
				t.Fatal(err)
			}
		}
	}
	expires := now.Add(time.Hour)
	root := metadata.Root(expires)
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": "alpha", "ardents_network": "alpha-network-1"}
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
		root.Signed.Keys[id], keyIDs = public, append(keyIDs, id)
	}
	for _, role := range metadata.TOP_LEVEL_ROLE_NAMES {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), keyIDs...), Threshold: 3}
	}
	sign(root, 5)
	rootBytes, err := root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(artifact)
	custom, err := json.Marshal(map[string]any{"schema_version": 1, "profile": "ardents-h3-release-v1", "platform": platform, "architecture": runtime.GOARCH,
		"environment": "alpha", "network": "alpha-network-1", "release_identity": "ardents-alpha-1", "release_version": 1, "source_revision": "test-source",
		"build_input_commitment": "test-inputs", "build_identity": "test-build", "dependency_identity": "test-dependencies", "sbom_identity": "test-sbom",
		"attestation_policy": "two-builder", "qualification": "qualified", "build_state": "current", "protocol_phase": "required", "protocol_overlapped_since": now.Add(-48 * time.Hour),
		"capacity_ready": true, "drain_ready": true, "build_safety_no_new_work_after": now.Add(20 * time.Minute), "build_safety_terminate_after": now.Add(40 * time.Minute),
		"builder_attestations": []map[string]string{{"builder_identity": "builder-a", "build_identity": "test-build", "source_revision": "test-source", "build_input_commitment": "test-inputs", "target_sha256": hex.EncodeToString(digest[:])},
			{"builder_identity": "builder-b", "build_identity": "test-build", "source_revision": "test-source", "build_input_commitment": "test-inputs", "target_sha256": hex.EncodeToString(digest[:])}}})
	if err != nil {
		t.Fatal(err)
	}
	targets := metadata.Targets(expires)
	targets.Signed.Targets[targetPath] = &metadata.TargetFiles{Length: int64(len(artifact)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: targetPath, Custom: (*json.RawMessage)(&custom)}
	sign(targets, 3)
	targetsBytes, err := targets.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	targetsDigest := sha256.Sum256(targetsBytes)
	snapshot := metadata.Snapshot(expires)
	snapshot.Signed.Meta["targets.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(targetsBytes)), Hashes: metadata.Hashes{"sha256": targetsDigest[:]}}
	sign(snapshot, 3)
	snapshotBytes, err := snapshot.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	timestamp := metadata.Timestamp(expires)
	timestamp.Signed.Meta["snapshot.json"] = &metadata.MetaFiles{Version: 1, Length: int64(len(snapshotBytes)), Hashes: metadata.Hashes{"sha256": snapshotDigest[:]}}
	sign(timestamp, 3)
	timestampBytes, err := timestamp.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"timestamp.json": timestampBytes, "1.snapshot.json": snapshotBytes, "1.targets.json": targetsBytes}, rootBytes
}

func referenceC2AlphaEpoch(network [32]byte, now time.Time, signer ed25519.PrivateKey) ([]byte, [32]byte) {
	inputRoot, viewRoot, rejectedRoot := sha256.Sum256([]byte{0x10}), sha256.Sum256([]byte{0x11}), sha256.Sum256([]byte{0x12})
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(1)
	raw.Write(network[:])
	referenceC2AlphaU64(&raw, 1)
	raw.Write(make([]byte, 32))
	referenceC2AlphaU64(&raw, uint64(now.Add(-time.Minute).Unix()))
	referenceC2AlphaU64(&raw, uint64(now.Add(10*time.Minute).Unix()))
	referenceC2AlphaU32(&raw, 0)
	referenceC2AlphaText(&raw, "h3-role-probe-v1")
	raw.Write(inputRoot[:])
	raw.Write(viewRoot[:])
	referenceC2AlphaU32(&raw, 0)
	raw.Write(rejectedRoot[:])
	referenceC2AlphaU32(&raw, 0)
	raw.Write(make([]byte, 32))
	referenceC2AlphaText(&raw, "ardents-h3-role-domain-v1")
	referenceC2AlphaU32(&raw, 0)
	referenceC2AlphaU32(&raw, 0)
	referenceC2AlphaU16(&raw, 0)
	referenceC2AlphaU16(&raw, 0)
	referenceC2AlphaU32(&raw, 0)
	raw.WriteByte(1)
	referenceC2AlphaText(&raw, "alpha")
	referenceC2AlphaU16(&raw, 0)
	referenceC2AlphaU32(&raw, 0)
	unsigned := raw.Bytes()
	digest := sha256.Sum256(unsigned)
	id := sha256.Sum256(signer.Public().(ed25519.PublicKey))
	raw.WriteByte(1)
	raw.Write(id[:])
	raw.Write(ed25519.Sign(signer, digest[:]))
	return raw.Bytes(), digest
}

func referenceC2AlphaManifest(t *testing.T, names []string, files map[string][]byte) []byte {
	t.Helper()
	lines := make([]string, 0, len(names))
	for _, name := range names {
		value, found := files[name]
		if !found {
			t.Fatalf("C2 alpha fixture missing %q", name)
		}
		digest := sha256.Sum256(value)
		lines = append(lines, hex.EncodeToString(digest[:])+"  "+name)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func referenceC2AlphaWrite(t *testing.T, path string, value []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, value, mode); err != nil {
		t.Fatal(err)
	}
}

func referenceC2AlphaText(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func referenceC2AlphaU16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}

func referenceC2AlphaU32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}

func referenceC2AlphaU64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}

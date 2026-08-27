//go:build linux

package endpoint_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
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
)

func TestAlphaControlReaderVerifiesPinnedBundleAndCachedRestart(t *testing.T) {
	endpoint := buildArdents(t)
	control := buildControl(t)
	fixture := alphaControlBundle(t, endpoint, control)
	state := t.TempDir()
	arguments := []string{"inspect-bundle", "--enrollment", fixture.input, "--artifact", fixture.artifact, "--state-root", state, "--at", fixture.now.Format(time.RFC3339)}
	for attempt := 0; attempt < 2; attempt++ {
		output, err := exec.Command(control, arguments...).CombinedOutput()
		if err != nil {
			t.Fatalf("alpha control inspect attempt %d: %v\n%s", attempt, err, output)
		}
		var report struct {
			Catalog      string `json:"catalog"`
			Release      string `json:"release"`
			NetworkEpoch uint64 `json:"network_epoch"`
			Components   [3]struct {
				Outcome string `json:"outcome"`
			} `json:"components"`
		}
		if err := json.Unmarshal(output, &report); err != nil || report.Catalog != "accepted" ||
			(report.Release != "release-accepted" && report.Release != "no-update") || report.NetworkEpoch != 1 {
			t.Fatalf("alpha control report attempt %d = %s / %+v / %v", attempt, output, report, err)
		}
		for _, component := range report.Components {
			if component.Outcome != "accepted" {
				t.Fatalf("alpha control component attempt %d = %+v", attempt, report.Components)
			}
		}
	}
	if _, err := os.Stat(fixture.bundle); err != nil {
		t.Fatal(err)
	}
}

func TestAlphaControlReaderTwoFreshEnrolledEndpointsAgree(t *testing.T) {
	endpoint := buildArdents(t)
	control := buildControl(t)
	fixture := alphaControlBundle(t, endpoint, control)
	arguments := []string{"inspect-bundle", "--enrollment", fixture.input, "--artifact", fixture.artifact,
		"--at", fixture.now.Format(time.RFC3339)}
	for endpointIndex := 0; endpointIndex < 2; endpointIndex++ {
		state := filepath.Join(t.TempDir(), "control-floor")
		output, err := exec.Command(control, append(arguments, "--state-root", state)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fresh enrolled Endpoint %d alpha control inspection: %v\n%s", endpointIndex, err, output)
		}
		var report struct {
			Catalog      string `json:"catalog"`
			Release      string `json:"release"`
			NetworkEpoch uint64 `json:"network_epoch"`
		}
		if err := json.Unmarshal(output, &report); err != nil || report.Catalog != "accepted" ||
			(report.Release != "release-accepted" && report.Release != "no-update") || report.NetworkEpoch != 1 {
			t.Fatalf("fresh enrolled Endpoint %d alpha control report = %s / %+v / %v", endpointIndex, output, report, err)
		}
	}
}

func TestAlphaCorpusAcceptanceUsesV3EnrolledControlCompanion(t *testing.T) {
	endpoint := buildArdents(t)
	control := buildControl(t)
	fixture := alphaControlBundle(t, endpoint, control)
	link, err := alpha.ParseServiceLink("ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-cohort-1", Network: fixture.network, Serial: 4,
		Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{7}}}, NotBefore: fixture.now.Add(-time.Minute), NotAfter: fixture.now.Add(10 * time.Minute)}, fixture.corpusPrivate)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	catalogPath, corpusPath := filepath.Join(directory, "catalog.ac2"), filepath.Join(directory, "corpus.anc")
	writeEnrollmentFile(t, catalogPath, alphaCorpusCatalog(t, fixture, 4, corpus), 0o600)
	writeEnrollmentFile(t, corpusPath, corpus, 0o600)
	controlRoot, corpusRoot := filepath.Join(directory, "control"), filepath.Join(directory, "corpus-floor")
	arguments := []string{"accept-alpha-corpus", "--enrollment", fixture.input, "--artifact", fixture.artifact,
		"--control-state-root", controlRoot, "--corpus-state-root", corpusRoot, "--catalog", catalogPath, "--corpus", corpusPath,
		"--at", fixture.now.Format(time.RFC3339)}
	if output, commandErr := exec.Command(control, arguments...).CombinedOutput(); commandErr == nil {
		t.Fatalf("outside alpha control command unexpectedly accepted enrolled corpus: %s", output)
	}
	for attempt := 0; attempt < 2; attempt++ {
		output, commandErr := exec.Command(fixture.control, arguments...).CombinedOutput()
		if commandErr != nil {
			t.Fatalf("alpha corpus acceptance attempt %d: %v\n%s", attempt, commandErr, output)
		}
		var report struct {
			Schema  string `json:"schema"`
			Corpus  string `json:"corpus"`
			Network string `json:"network"`
			Serial  uint64 `json:"serial"`
		}
		if err := json.Unmarshal(output, &report); err != nil || report.Schema != "ardents-alpha-corpus-acceptance-v1" ||
			report.Corpus != "accepted" || report.Network != hex.EncodeToString(fixture.network[:]) || report.Serial != 4 {
			t.Fatalf("alpha corpus acceptance attempt %d = %s / %+v / %v", attempt, output, report, err)
		}
	}
	successor, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-cohort-1", Network: fixture.network, Serial: 5,
		Bindings: []alpha.BindingInput{{Link: link, Target: [32]byte{8}}}, NotBefore: fixture.now.Add(-time.Minute), NotAfter: fixture.now.Add(10 * time.Minute)}, fixture.corpusPrivate)
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, catalogPath, alphaCorpusCatalog(t, fixture, 5, successor), 0o600)
	writeEnrollmentFile(t, corpusPath, successor, 0o600)
	output, commandErr := exec.Command(fixture.control, arguments...).CombinedOutput()
	if commandErr != nil {
		t.Fatalf("alpha corpus successor acceptance: %v\n%s", commandErr, output)
	}
	var successorReport struct {
		Serial uint64 `json:"serial"`
	}
	if err := json.Unmarshal(output, &successorReport); err != nil || successorReport.Serial != 5 {
		t.Fatalf("alpha corpus successor report = %s / %+v / %v", output, successorReport, err)
	}
	writeEnrollmentFile(t, catalogPath, alphaCorpusCatalog(t, fixture, 4, corpus), 0o600)
	writeEnrollmentFile(t, corpusPath, corpus, 0o600)
	if output, commandErr := exec.Command(fixture.control, arguments...).CombinedOutput(); commandErr == nil {
		t.Fatalf("alpha corpus rollback unexpectedly accepted: %s", output)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: corpusRoot, Authority: fixture.corpusPublic, Cohort: "closed-cohort-1", Network: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	current, err := floor.Current()
	if err != nil || current.Serial() != 5 {
		t.Fatalf("alpha corpus floor after replacement = %+v / %v", current, err)
	}
	binding, err := current.Resolve(link, fixture.now)
	if err != nil || binding.Target() != [32]byte{8} {
		t.Fatalf("alpha corpus successor binding = %+v / %v", binding, err)
	}
}

func alphaCorpusCatalog(t *testing.T, fixture alphaControlBundleFixture, serial uint64, corpus []byte) []byte {
	t.Helper()
	catalog := alphacontrol.CatalogV2{Cohort: "closed-cohort-1", Generation: 1, NotBefore: fixture.now.Add(-time.Minute), NotAfter: fixture.now.Add(20 * time.Minute)}
	for index := 0; index < 3; index++ {
		body := []byte{byte(index + 1)}
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: [32]byte{byte(index + 1)},
			Generation: 1, NotAfter: catalog.NotAfter, Size: uint32(len(body)), Digest: sha256.Sum256(body)}
	}
	catalog.Components[3] = alphacontrol.Component{Class: alphacontrol.ComponentCorpus, RootID: sha256.Sum256(fixture.corpusPublic),
		Generation: serial, NotAfter: fixture.now.Add(10 * time.Minute), Size: uint32(len(corpus)), Digest: sha256.Sum256(corpus)}
	raw, err := alphacontrol.SignV2(catalog, fixture.disclosurePrivate)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func buildControl(t *testing.T) string {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_E2E_CONTROL"); prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt alpha-control command is not a regular file: %v", err)
		}
		return prebuilt
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E source")
	}
	path := filepath.Join(t.TempDir(), "ardents-control")
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", path, "./cmd/ardents-control")
	build.Dir = filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build control reader: %v\n%s", err, output)
	}
	return path
}

type alphaControlBundleFixture struct {
	bundle, artifact, control, input string
	disclosurePrivate                ed25519.PrivateKey
	corpusPublic                     ed25519.PublicKey
	corpusPrivate                    ed25519.PrivateKey
	network                          [32]byte
	now                              time.Time
}

func alphaControlBundle(t *testing.T, endpoint, control string) alphaControlBundleFixture {
	t.Helper()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	artifactName, controlName, targetPath := "ardents-"+platform, "ardents-control-"+platform, "ardents/"+platform+"/endpoint"
	artifact, err := os.ReadFile(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	controlArtifact, err := os.ReadFile(control)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	metadataFiles, rootBytes := enrolledRuntimeMetadata(t, artifact, targetPath, platform, now)
	decision := alphaControlReleaseDecision(t, rootBytes, metadataFiles, artifact, targetPath, platform, now)
	components, roots, catalogRoot, catalog, disclosurePrivate, network := alphaControlStatements(t, decision, now)
	corpusPublic, corpusPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v3", "cohort=closed-cohort-1", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json",
		"control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1", "control_compatibility=compatibility.ac1",
		"control_release_root=release.pub", "control_network_root=network.pub", "control_compatibility_root=compatibility.pub", "corpus_authority=corpus.pub", "control_artifact=" + controlName,
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact, controlName: controlArtifact, "catalog.ac1": catalog, "catalog.pub": catalogRoot,
		"release.ac1": components[0], "network.ac1": components[1], "compatibility.ac1": components[2], "release.pub": roots[0], "network.pub": roots[1], "compatibility.pub": roots[2], "corpus.pub": corpusPublic}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	names := []string{"1.root.json", "1.snapshot.json", "1.targets.json", "RELEASE", controlName, artifactName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "corpus.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"}
	for _, name := range names {
		mode := os.FileMode(0o600)
		if name == artifactName || name == controlName {
			mode = 0o700
		}
		writeEnrollmentFile(t, filepath.Join(bundle, name), files[name], mode)
	}
	manifest := enrolledRuntimeManifest(t, names, files)
	writeEnrollmentFile(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	pin := sha256.Sum256(manifest)
	input := filepath.Join(t.TempDir(), "alpha-enrollment.json")
	raw, err := json.Marshal(map[string]string{"schema": "ardents-alpha-enrollment-input-v1", "bundle_root": bundle, "cohort": "closed-cohort-1", "release": "alpha-1",
		"platform": platform, "manifest_sha256": hex.EncodeToString(pin[:]), "environment": "alpha", "network": "alpha-network-1", "target_path": targetPath})
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, input, raw, 0o600)
	return alphaControlBundleFixture{bundle: bundle, artifact: filepath.Join(bundle, artifactName), control: filepath.Join(bundle, controlName), input: input,
		disclosurePrivate: disclosurePrivate, corpusPublic: corpusPublic, corpusPrivate: corpusPrivate, network: network, now: now}
}

func alphaControlReleaseDecision(t *testing.T, root []byte, metadata map[string][]byte, artifact []byte, targetPath, platform string, now time.Time) release.Decision {
	t.Helper()
	verifier, err := release.Open(filepath.Join(t.TempDir(), "release"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = verifier.Close() })
	decision := verifier.Evaluate(context.Background(), release.Inputs{RootBytes: root, Files: map[string][]byte{
		release.MetadataURL("timestamp.json"): metadata["timestamp.json"], release.MetadataURL("1.snapshot.json"): metadata["1.snapshot.json"], release.MetadataURL("1.targets.json"): metadata["1.targets.json"]},
		TargetPath: targetPath, Artifact: artifact, Local: release.LocalEnvironment{Environment: "alpha", Network: "alpha-network-1", Platform: platform, Architecture: runtime.GOARCH, RefTime: now}})
	if decision.Outcome != release.OutcomeReleaseAccepted && decision.Outcome != release.OutcomeNoUpdate {
		t.Fatalf("alpha control release decision = %+v", decision)
	}
	return decision
}

func alphaControlStatements(t *testing.T, decision release.Decision, now time.Time) ([3][]byte, [3][]byte, []byte, []byte, ed25519.PrivateKey, [32]byte) {
	t.Helper()
	disclosurePublic, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
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
	networkID := [32]byte{9}
	epoch, epochDigest := alphaControlEpoch(networkID, now, authorityPrivate)
	networkBody, err := inspection.EncodeNetworkEvidence(inspection.NetworkEvidence{NetworkID: networkID, EpochDigest: epochDigest, Profile: "h3-role-probe-v1", Threshold: 1, Authorities: []ed25519.PublicKey{authority}, Epoch: epoch})
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
	catalog := alphacontrol.Catalog{Cohort: "closed-cohort-1", Generation: 1, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(20 * time.Minute)}
	for index, body := range bodies {
		public, private, keyErr := ed25519.GenerateKey(rand.Reader)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		components[index], keyErr = alphacontrol.SignComponent(alphacontrol.ComponentStatement{Class: alphacontrol.ComponentClass(index + 1), Generation: 1,
			NotBefore: now.Add(-time.Minute), NotAfter: catalog.NotAfter, Body: body}, private)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		roots[index] = append([]byte(nil), public...)
		catalog.Components[index] = alphacontrol.Component{Class: alphacontrol.ComponentClass(index + 1), RootID: sha256.Sum256(public), Generation: 1,
			NotAfter: catalog.NotAfter, Size: uint32(len(components[index])), Digest: sha256.Sum256(components[index])}
	}
	raw, err := alphacontrol.Sign(catalog, disclosurePrivate)
	if err != nil {
		t.Fatal(err)
	}
	return components, roots, disclosurePublic, raw, disclosurePrivate, networkID
}

func alphaControlEpoch(network [32]byte, now time.Time, signer ed25519.PrivateKey) ([]byte, [32]byte) {
	inputRoot, viewRoot, rejectedRoot := sha256.Sum256([]byte{0x10}), sha256.Sum256([]byte{0x11}), sha256.Sum256([]byte{0x12})
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(1)
	raw.Write(network[:])
	alphaControlU64(&raw, 1)
	raw.Write(make([]byte, 32))
	alphaControlU64(&raw, uint64(now.Add(-time.Minute).Unix()))
	alphaControlU64(&raw, uint64(now.Add(10*time.Minute).Unix()))
	alphaControlU32(&raw, 0)
	alphaControlText(&raw, "h3-role-probe-v1")
	raw.Write(inputRoot[:])
	raw.Write(viewRoot[:])
	alphaControlU32(&raw, 0)
	raw.Write(rejectedRoot[:])
	alphaControlU32(&raw, 0)
	raw.Write(make([]byte, 32))
	alphaControlText(&raw, "ardents-h3-role-domain-v1")
	alphaControlU32(&raw, 0)
	alphaControlU32(&raw, 0)
	alphaControlU16(&raw, 0)
	alphaControlU16(&raw, 0)
	alphaControlU32(&raw, 0)
	raw.WriteByte(1)
	alphaControlText(&raw, "alpha")
	alphaControlU16(&raw, 0)
	alphaControlU32(&raw, 0)
	unsigned := raw.Bytes()
	digest := sha256.Sum256(unsigned)
	public := signer.Public().(ed25519.PublicKey)
	id := sha256.Sum256(public)
	raw.WriteByte(1)
	raw.Write(id[:])
	raw.Write(ed25519.Sign(signer, digest[:]))
	return raw.Bytes(), digest
}

func alphaControlText(buffer *bytes.Buffer, value string) {
	buffer.WriteByte(byte(len(value)))
	buffer.WriteString(value)
}

func alphaControlU16(buffer *bytes.Buffer, value uint16) {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], value)
	buffer.Write(raw[:])
}

func alphaControlU32(buffer *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	buffer.Write(raw[:])
}

func alphaControlU64(buffer *bytes.Buffer, value uint64) {
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], value)
	buffer.Write(raw[:])
}

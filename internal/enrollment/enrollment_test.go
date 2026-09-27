package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestVerifyPinsExactBundleBeforeParsingAndBuildsReleaseInputs(t *testing.T) {
	root, request := enrolledFixture(t)
	verified, err := Verify(request)
	if err != nil {
		t.Fatal(err)
	}
	if string(verified.Inputs.RootBytes) != "trusted root" || verified.Inputs.TargetPath != "ardents/linux-amd64/endpoint" {
		t.Fatalf("Release inputs are not bound to the descriptor: %+v", verified.Inputs)
	}
	if got := string(verified.Inputs.Files[release.MetadataURL("timestamp.json")]); got != "timestamp" {
		t.Fatalf("timestamp metadata = %q", got)
	}
	if string(verified.ControlCatalog) != "catalog" || string(verified.DisclosureRoot) != "key" ||
		string(verified.ControlRelease) != "release control" || string(verified.ControlNetwork) != "network control" ||
		string(verified.ControlCompatibility) != "compatibility control" || string(verified.ControlReleaseRoot) != "release key" ||
		string(verified.ControlNetworkRoot) != "network key" || string(verified.ControlCompatibilityRoot) != "compatibility key" || verified.Inputs.Files[release.MetadataURL("catalog.ac1")] != nil ||
		verified.Inputs.Files[release.MetadataURL("release.ac1")] != nil || verified.Inputs.Files[release.MetadataURL("network.ac1")] != nil ||
		verified.Inputs.Files[release.MetadataURL("compatibility.ac1")] != nil || verified.Inputs.Files[release.MetadataURL("release.pub")] != nil ||
		verified.Inputs.Files[release.MetadataURL("network.pub")] != nil || verified.Inputs.Files[release.MetadataURL("compatibility.pub")] != nil ||
		verified.Inputs.Files[release.MetadataURL("corpus.pub")] != nil || verified.Inputs.Files[release.MetadataURL(fixtureControlName)] != nil ||
		verified.ControlArtifactName != fixtureControlName || !bytes.Equal(verified.ControlArtifact, fixtureControlArtifact) ||
		!bytes.Equal(verified.CorpusAuthority, fixtureCorpusAuthority) {
		t.Fatalf("alpha control companions crossed the Release boundary: %+v", verified)
	}
	if err := os.WriteFile(filepath.Join(root, manifestName), []byte("not a manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(request); err == nil || !strings.Contains(err.Error(), "independent pin") {
		t.Fatalf("changed manifest result = %v", err)
	}
}

func TestVerifyRejectsUnknownInventoryAndExecutableSubstitution(t *testing.T) {
	root, request := enrolledFixture(t)
	if err := os.WriteFile(filepath.Join(root, "extra"), []byte("extra"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(request); err == nil || !strings.Contains(err.Error(), "inventory") {
		t.Fatalf("unknown inventory result = %v", err)
	}
	if err := os.Remove(filepath.Join(root, "extra")); err != nil {
		t.Fatal(err)
	}
	request.ExecutablePath = filepath.Join(root, "1.root.json")
	if _, err := Verify(request); err == nil || !strings.Contains(err.Error(), "running executable") {
		t.Fatalf("substituted executable result = %v", err)
	}
}

func TestVerifyRefusesRetiredDescriptorVersions(t *testing.T) {
	// ADR-0112 evidence: a recognized retired Network schema is refused with
	// the typed sentinel through both entry points, even over a consistent
	// manifest whose pin matches.
	for _, schema := range []string{"v1", "v2"} {
		_, request := enrolledFixtureWithSchema(t, schema)
		if _, err := Verify(request); !errors.Is(err, ErrLegacyEnrollmentDescriptor) {
			t.Fatalf("retired %s Verify = %v", schema, err)
		}
		if _, err := VerifyHeadless(request); !errors.Is(err, ErrLegacyEnrollmentDescriptor) {
			t.Fatalf("retired %s VerifyHeadless = %v", schema, err)
		}
	}
	// An unknown schema keeps its generic invalid refusal, not the typed one.
	_, unknown := enrolledFixtureWithSchema(t, "v4")
	if _, err := Verify(unknown); err == nil || errors.Is(err, ErrLegacyEnrollmentDescriptor) {
		t.Fatalf("unknown schema result = %v", err)
	}
	// The manifest pin still precedes descriptor parsing: a retired bundle
	// with a mismatched pin reports the pin failure, not the version.
	root, retired := enrolledFixtureWithSchema(t, "v1")
	if err := os.WriteFile(filepath.Join(root, manifestName), []byte("not a manifest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(retired); err == nil || !strings.Contains(err.Error(), "independent pin") {
		t.Fatalf("pin precedence over the typed refusal = %v", err)
	}
}

func TestExecutableArtifactNameIsCanonicalForEveryEnrollmentPlatform(t *testing.T) {
	for _, test := range []struct {
		platform string
		want     string
	}{
		{platform: "linux-amd64", want: "ardents-control-linux-amd64"},
		{platform: "windows-amd64", want: "ardents-control-windows-amd64.exe"},
	} {
		if got := ExecutableArtifactName("ardents-control", test.platform); got != test.want {
			t.Fatalf("ExecutableArtifactName(ardents-control, %q) = %q, want %q", test.platform, got, test.want)
		}
	}
}

const windowsV3VerifierChild = "ARDENTS_WINDOWS_V3_VERIFIER_CHILD"

func TestWindowsV3ManifestVerifyPinsTheRunningArtifactIdentity(t *testing.T) {
	if os.Getenv(windowsV3VerifierChild) == "1" {
		var request Request
		if err := json.Unmarshal([]byte(os.Getenv("ARDENTS_WINDOWS_V3_REQUEST")), &request); err != nil {
			t.Fatal(err)
		}
		running, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		request.ExecutablePath = running
		// Verify's own running-artifact gate (exactExecutable) must accept
		// this process as the exact enrolled endpoint artifact; the retired
		// companion verifier (ADR-0113) added no separate authority.
		if _, err := Verify(request); err != nil {
			t.Fatal(err)
		}
		return
	}

	const platform = "windows-amd64"
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	endpointName := ExecutableArtifactName("ardents", platform)
	controlName := ExecutableArtifactName("ardents-control", platform)
	files := map[string][]byte{
		"1.root.json": []byte("trusted root"), "catalog.ac1": []byte("catalog"), "catalog.pub": []byte("key"),
		"compatibility.ac1": []byte("compatibility control"), "compatibility.pub": []byte("compatibility key"),
		"corpus.pub": bytes.Repeat([]byte{9}, 32), "network.ac1": []byte("network control"), "network.pub": []byte("network key"),
		"release.ac1": []byte("release control"), "release.pub": []byte("release key"), "timestamp.json": []byte("timestamp"),
		endpointName: program, controlName: program,
	}
	files[descriptorName] = []byte(strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v3", "cohort=cohort-1", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=network-1", "target_path=ardents/windows-amd64/endpoint", "artifact=" + endpointName,
		"trusted_root=1.root.json", "control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1",
		"control_network=network.ac1", "control_compatibility=compatibility.ac1", "control_release_root=release.pub",
		"control_network_root=network.pub", "control_compatibility_root=compatibility.pub", "corpus_authority=corpus.pub",
		"control_artifact=" + controlName,
	}, "\n") + "\n")
	for name, contents := range files {
		mode := os.FileMode(0o600)
		if name == endpointName || name == controlName {
			mode = 0o700
		}
		if err := os.WriteFile(filepath.Join(root, name), contents, mode); err != nil {
			t.Fatal(err)
		}
	}
	manifest := makeManifest(t, files)
	if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(manifest)
	request := Request{BundleRoot: root, ExecutablePath: filepath.Join(root, endpointName), Pin: Pin{Cohort: "cohort-1", Release: "alpha-1", Platform: platform, ManifestSHA256: hex.EncodeToString(pin[:])},
		Environment: "alpha", Network: "network-1", TargetPath: "ardents/windows-amd64/endpoint", Architecture: "amd64", ReferenceTime: time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(root, endpointName), "-test.run=^TestWindowsV3ManifestVerifyPinsTheRunningArtifactIdentity$")
	command.Env = append(os.Environ(), windowsV3VerifierChild+"=1", "ARDENTS_WINDOWS_V3_REQUEST="+string(encoded))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Windows enrollment-v3 verifier contract: %v\n%s", err, output)
	}
}

func TestVerifyReturnsV3HeadlessArtifactsOutsideReleaseMetadata(t *testing.T) {
	root, request := enrolledFixture(t)
	const nodeName = "ardents-node-linux-amd64"
	const custodyName = "ardents-custody-linux-amd64"
	node, custody := []byte("separately manifested Network Node command"), []byte("separately manifested Authority Custody command")
	names := []string{"1.root.json", "RELEASE", "ardents-linux-amd64", fixtureControlName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "corpus.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"}
	files := make(map[string][]byte)
	for _, name := range names {
		contents, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[name] = contents
	}
	repin := func() {
		t.Helper()
		manifest := makeManifest(t, files)
		if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
			t.Fatal(err)
		}
		pinned := sha256.Sum256(manifest)
		request.Pin.ManifestSHA256 = hex.EncodeToString(pinned[:])
	}
	for name, contents := range map[string][]byte{nodeName: node, custodyName: custody} {
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o700); err != nil {
			t.Fatal(err)
		}
		files[name] = contents
	}
	repin()
	verified, err := VerifyHeadless(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(verified.CorpusAuthority, fixtureCorpusAuthority) || verified.ControlArtifactName != fixtureControlName ||
		!bytes.Equal(verified.ControlArtifact, fixtureControlArtifact) || verified.NodeArtifactName != nodeName || !bytes.Equal(verified.NodeArtifact, node) ||
		verified.CustodyArtifactName != custodyName || !bytes.Equal(verified.CustodyArtifact, custody) ||
		verified.Inputs.Files[release.MetadataURL("corpus.pub")] != nil || verified.Inputs.Files[release.MetadataURL(fixtureControlName)] != nil ||
		verified.Inputs.Files[release.MetadataURL(nodeName)] != nil || verified.Inputs.Files[release.MetadataURL(custodyName)] != nil {
		t.Fatalf("v3 control artifact crossed an incorrect boundary: %+v", verified)
	}
	// A partial companion pair fails closed.
	if err := os.Remove(filepath.Join(root, nodeName)); err != nil {
		t.Fatal(err)
	}
	delete(files, nodeName)
	repin()
	if _, err := Verify(request); err == nil || !strings.Contains(err.Error(), "partial headless companion") {
		t.Fatalf("partial companion pair result = %v", err)
	}
	// The accepted v3 inventory without companions still verifies generally,
	// while the headless gate refuses it.
	if err := os.Remove(filepath.Join(root, custodyName)); err != nil {
		t.Fatal(err)
	}
	delete(files, custodyName)
	repin()
	if _, err := Verify(request); err != nil {
		t.Fatalf("accepted ADR-0042 v3 inventory no longer verifies: %v", err)
	}
	if _, err := VerifyHeadless(request); err == nil {
		t.Fatal("headless candidate accepted v3 without Node and custody companions")
	}
}

const fixtureControlName = "ardents-control-linux-amd64"

var (
	fixtureCorpusAuthority = bytes.Repeat([]byte{9}, 32)
	fixtureControlArtifact = []byte("separately manifested alpha control command")
)

func enrolledFixture(t *testing.T) (string, Request) {
	t.Helper()
	return enrolledFixtureWithSchema(t, "v3")
}

// enrolledFixtureWithSchema builds one bundle with a consistent manifest and
// independent pin for the named Network enrollment descriptor schema. Only the
// v3 grammar verifies; the retired v1/v2 forms exist as typed-refusal evidence
// and an unknown schema keeps its generic refusal (ADR-0112).
func enrolledFixtureWithSchema(t *testing.T, schema string) (string, Request) {
	t.Helper()
	root := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(root, "ardents-linux-amd64")
	artifact, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		"schema=ardents-closed-alpha-enrollment-" + schema,
		"cohort=cohort-1",
		"release=alpha-1",
		"platform=linux-amd64",
		"environment=alpha",
		"network=network-1",
		"target_path=ardents/linux-amd64/endpoint",
		"artifact=ardents-linux-amd64",
		"trusted_root=1.root.json",
		"control_catalog=catalog.ac1",
		"disclosure_root=catalog.pub",
		"control_release=release.ac1",
		"control_network=network.ac1",
		"control_compatibility=compatibility.ac1",
		"control_release_root=release.pub",
		"control_network_root=network.pub",
		"control_compatibility_root=compatibility.pub",
	}
	files := map[string][]byte{"1.root.json": []byte("trusted root"), "timestamp.json": []byte("timestamp"), "catalog.ac1": []byte("catalog"), "catalog.pub": []byte("key"), "release.ac1": []byte("release control"), "network.ac1": []byte("network control"), "compatibility.ac1": []byte("compatibility control"), "release.pub": []byte("release key"), "network.pub": []byte("network key"), "compatibility.pub": []byte("compatibility key")}
	if schema == "v2" || schema == "v3" {
		lines = append(lines, "corpus_authority=corpus.pub")
		files["corpus.pub"] = fixtureCorpusAuthority
	}
	if schema == "v3" {
		lines = append(lines, "control_artifact="+fixtureControlName)
		files[fixtureControlName] = fixtureControlArtifact
	}
	files[descriptorName] = []byte(strings.Join(lines, "\n") + "\n")
	for name, contents := range files {
		mode := os.FileMode(0o600)
		if name == fixtureControlName {
			mode = 0o700
		}
		if err := os.WriteFile(filepath.Join(root, name), contents, mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(artifactPath, artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	files["ardents-linux-amd64"] = artifact
	manifest := makeManifest(t, files)
	if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pinned := sha256.Sum256(manifest)
	return root, Request{BundleRoot: root, ExecutablePath: artifactPath,
		Pin:         Pin{Cohort: "cohort-1", Release: "alpha-1", Platform: "linux-amd64", ManifestSHA256: hex.EncodeToString(pinned[:])},
		Environment: "alpha", Network: "network-1", TargetPath: "ardents/linux-amd64/endpoint", Architecture: "amd64",
		ReferenceTime: time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)}
}

func makeManifest(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		digest := sha256.Sum256(files[name])
		lines = append(lines, hex.EncodeToString(digest[:])+"  "+name)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

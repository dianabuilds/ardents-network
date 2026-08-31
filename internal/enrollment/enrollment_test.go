package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
		verified.Inputs.Files[release.MetadataURL("network.pub")] != nil || verified.Inputs.Files[release.MetadataURL("compatibility.pub")] != nil {
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

func TestVerifyReturnsV2IndependentlyPinnedCorpusAuthority(t *testing.T) {
	root, request := enrolledFixture(t)
	authority := bytes.Repeat([]byte{9}, 32)
	if err := os.WriteFile(filepath.Join(root, "corpus.pub"), authority, 0o600); err != nil {
		t.Fatal(err)
	}
	descriptorPath := filepath.Join(root, descriptorName)
	descriptor, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor = bytes.Replace(descriptor, []byte("schema=ardents-closed-alpha-enrollment-v1"), []byte("schema=ardents-closed-alpha-enrollment-v2"), 1)
	descriptor = append(descriptor[:len(descriptor)-1], []byte("\ncorpus_authority=corpus.pub\n")...)
	if err := os.WriteFile(descriptorPath, descriptor, 0o600); err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, name := range []string{"1.root.json", "RELEASE", "ardents-linux-amd64", "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "corpus.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"} {
		contents, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[name] = contents
	}
	manifest := makeManifest(t, files)
	if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pinned := sha256.Sum256(manifest)
	request.Pin.ManifestSHA256 = hex.EncodeToString(pinned[:])
	verified, err := Verify(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(verified.CorpusAuthority, authority) || verified.Inputs.Files[release.MetadataURL("corpus.pub")] != nil {
		t.Fatalf("v2 corpus authority crossed an incorrect boundary: %+v", verified)
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

func TestWindowsV3ManifestAndRunningCompanionShareArtifactIdentity(t *testing.T) {
	if os.Getenv(windowsV3VerifierChild) == "1" {
		var request Request
		if err := json.Unmarshal([]byte(os.Getenv("ARDENTS_WINDOWS_V3_REQUEST")), &request); err != nil {
			t.Fatal(err)
		}
		verified, err := Verify(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyRunningCompanion(request, verified.ControlArtifactName, verified.ControlArtifact); err != nil {
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
	command := exec.Command(filepath.Join(root, controlName), "-test.run=^TestWindowsV3ManifestAndRunningCompanionShareArtifactIdentity$")
	command.Env = append(os.Environ(), windowsV3VerifierChild+"=1", "ARDENTS_WINDOWS_V3_REQUEST="+string(encoded))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Windows enrollment-v3 verifier contract: %v\n%s", err, output)
	}
}

func TestVerifyReturnsV3HeadlessArtifactsOutsideReleaseMetadata(t *testing.T) {
	root, request := enrolledFixture(t)
	authority, control := bytes.Repeat([]byte{9}, 32), []byte("separately manifested alpha control command")
	node, custody := []byte("separately manifested Network Node command"), []byte("separately manifested Authority Custody command")
	if err := os.WriteFile(filepath.Join(root, "corpus.pub"), authority, 0o600); err != nil {
		t.Fatal(err)
	}
	const controlName = "ardents-control-linux-amd64"
	const nodeName = "ardents-node-linux-amd64"
	const custodyName = "ardents-custody-linux-amd64"
	for name, contents := range map[string][]byte{controlName: control, nodeName: node, custodyName: custody} {
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	descriptorPath := filepath.Join(root, descriptorName)
	descriptor, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatal(err)
	}
	descriptor = bytes.Replace(descriptor, []byte("schema=ardents-closed-alpha-enrollment-v1"), []byte("schema=ardents-closed-alpha-enrollment-v3"), 1)
	descriptor = append(descriptor[:len(descriptor)-1], []byte("\ncorpus_authority=corpus.pub\ncontrol_artifact="+controlName+"\n")...)
	if err := os.WriteFile(descriptorPath, descriptor, 0o600); err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte)
	for _, name := range []string{"1.root.json", "RELEASE", "ardents-linux-amd64", controlName, nodeName, custodyName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "corpus.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"} {
		contents, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[name] = contents
	}
	manifest := makeManifest(t, files)
	if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pinned := sha256.Sum256(manifest)
	request.Pin.ManifestSHA256 = hex.EncodeToString(pinned[:])
	verified, err := VerifyHeadless(request)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(verified.CorpusAuthority, authority) || verified.ControlArtifactName != controlName ||
		!bytes.Equal(verified.ControlArtifact, control) || verified.NodeArtifactName != nodeName || !bytes.Equal(verified.NodeArtifact, node) ||
		verified.CustodyArtifactName != custodyName || !bytes.Equal(verified.CustodyArtifact, custody) ||
		verified.Inputs.Files[release.MetadataURL(controlName)] != nil || verified.Inputs.Files[release.MetadataURL(nodeName)] != nil ||
		verified.Inputs.Files[release.MetadataURL(custodyName)] != nil {
		t.Fatalf("v3 control artifact crossed an incorrect boundary: %+v", verified)
	}
	delete(files, nodeName)
	delete(files, custodyName)
	if err := os.Remove(filepath.Join(root, nodeName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, custodyName)); err != nil {
		t.Fatal(err)
	}
	legacyManifest := makeManifest(t, files)
	if err := os.WriteFile(filepath.Join(root, manifestName), legacyManifest, 0o600); err != nil {
		t.Fatal(err)
	}
	legacyPin := sha256.Sum256(legacyManifest)
	request.Pin.ManifestSHA256 = hex.EncodeToString(legacyPin[:])
	if _, err := Verify(request); err != nil {
		t.Fatalf("accepted ADR-0042 v3 inventory no longer verifies: %v", err)
	}
	if _, err := VerifyHeadless(request); err == nil {
		t.Fatal("headless candidate accepted v3 without Node and custody companions")
	}
}

func TestVerifyBindsPackageOwnedArtifactOutsideStaticEnrollmentDirectory(t *testing.T) {
	root, request := enrolledFixture(t)
	artifactName := "ardents-linux-amd64"
	artifact, err := os.ReadFile(filepath.Join(root, artifactName))
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(t.TempDir(), "usr", "lib", "ardents", "ardents")
	if err := os.MkdirAll(filepath.Dir(installed), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, artifactName)); err != nil {
		t.Fatal(err)
	}
	request.ExecutablePath = installed
	request.ArtifactPath = installed
	if _, err := Verify(request); err != nil {
		t.Fatalf("Verify(package enrollment) = %v", err)
	}
	if err := os.WriteFile(installed, []byte("substituted installed program"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(request); err == nil {
		t.Fatalf("substituted installed program result = %v", err)
	}
}

func enrolledFixture(t *testing.T) (string, Request) {
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
	if err := os.WriteFile(artifactPath, artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v1",
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
	}, "\n") + "\n"
	files := map[string][]byte{"RELEASE": []byte(descriptor), "1.root.json": []byte("trusted root"), "timestamp.json": []byte("timestamp"), "catalog.ac1": []byte("catalog"), "catalog.pub": []byte("key"), "release.ac1": []byte("release control"), "network.ac1": []byte("network control"), "compatibility.ac1": []byte("compatibility control"), "release.pub": []byte("release key"), "network.pub": []byte("network key"), "compatibility.pub": []byte("compatibility key")}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
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

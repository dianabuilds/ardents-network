package enrollment

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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
	names := []string{"1.root.json", "RELEASE", "ardents-linux-amd64", "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"}
	lines := make([]string, 0, len(names))
	for _, name := range names {
		digest := sha256.Sum256(files[name])
		lines = append(lines, hex.EncodeToString(digest[:])+"  "+name)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

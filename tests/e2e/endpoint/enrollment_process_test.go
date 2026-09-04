package endpoint_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnrollmentCheckAcceptsExactRunningBundleAndRejectsChangedManifest(t *testing.T) {
	command := buildArdents(t)
	bundle, enrolledCommand, manifestPin := enrollmentBundle(t, command)

	output, err := exec.Command(enrolledCommand, "endpoint", "enrollment-check", bundle, manifestPin).CombinedOutput()
	if err != nil {
		t.Fatalf("verify exact enrolled bundle: %v\n%s", err, output)
	}
	var observed struct {
		Kind, Cohort, Release, Platform, ArtifactSHA256 string
	}
	if err := json.Unmarshal(output, &observed); err != nil || observed.Kind != "manifest-pinned-enrollment-verified" ||
		observed.Cohort != "closed-cohort-1" || observed.Release != "alpha-1" || observed.Platform != runtime.GOOS+"-"+runtime.GOARCH ||
		len(observed.ArtifactSHA256) != sha256.Size*2 {
		t.Fatalf("exact bundle result = %q / %+v / %v", output, observed, err)
	}

	manifest := filepath.Join(bundle, "SHA256SUMS")
	if err := os.WriteFile(manifest, []byte("changed-before-parse\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err = exec.Command(enrolledCommand, "endpoint", "enrollment-check", bundle, manifestPin).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "independent pin") {
		t.Fatalf("changed manifest result: err=%v output=%s", err, output)
	}
}

func TestManifestPinnedEnrollmentArgumentsReadsOnlyFixtureMetadata(t *testing.T) {
	input := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(input, []byte(`{"bundle_root":"/bundle","manifest_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bundleRoot, manifestPin := manifestPinnedEnrollmentArguments(t, input)
	if bundleRoot != "/bundle" || manifestPin != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("fixture arguments = %q, %q", bundleRoot, manifestPin)
	}
}

func buildArdents(t *testing.T) string {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_E2E_COMMAND"); prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt Ardents command is not a regular file: %v", err)
		}
		return prebuilt
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E source")
	}
	name := "ardents"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", path, "./cmd/ardents")
	build.Dir = filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build Ardents command: %v\n%s", err, output)
	}
	return path
}

func enrollmentBundle(t *testing.T, command string) (string, string, string) {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	artifactName := "ardents-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		artifactName += ".exe"
	}
	artifact, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, filepath.Join(bundle, artifactName), artifact, 0o700)
	platform := runtime.GOOS + "-" + runtime.GOARCH
	targetPath := "ardents/" + platform + "/endpoint"
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v1",
		"cohort=closed-cohort-1",
		"release=alpha-1",
		"platform=" + platform,
		"environment=alpha",
		"network=alpha-network-1",
		"target_path=" + targetPath,
		"artifact=" + artifactName,
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
	files := map[string][]byte{
		"1.root.json":       []byte("synthetic trusted root\n"),
		"RELEASE":           []byte(descriptor),
		"catalog.ac1":       []byte("catalog"),
		"catalog.pub":       []byte("key"),
		"release.ac1":       []byte("release control"),
		"network.ac1":       []byte("network control"),
		"compatibility.ac1": []byte("compatibility control"),
		"release.pub":       []byte("release key"),
		"network.pub":       []byte("network key"),
		"compatibility.pub": []byte("compatibility key"),
		artifactName:        artifact,
		"timestamp.json":    []byte("synthetic metadata\n"),
	}
	for name, contents := range files {
		if name != artifactName {
			writeEnrollmentFile(t, filepath.Join(bundle, name), contents, 0o600)
		}
	}
	names := []string{"1.root.json", "RELEASE", artifactName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"}
	lines := make([]string, 0, len(names))
	for _, name := range names {
		digest := sha256.Sum256(files[name])
		lines = append(lines, hex.EncodeToString(digest[:])+"  "+name)
	}
	manifest := []byte(strings.Join(lines, "\n") + "\n")
	writeEnrollmentFile(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	pinned := sha256.Sum256(manifest)
	return bundle, filepath.Join(bundle, artifactName), hex.EncodeToString(pinned[:])
}

// manifestPinnedEnrollmentArguments recovers test-fixture metadata only. The
// production command receives the independently pinned manifest digest, never
// this fixture file.
func manifestPinnedEnrollmentArguments(t *testing.T, input string) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var enrollment struct {
		BundleRoot     string `json:"bundle_root"`
		ManifestSHA256 string `json:"manifest_sha256"`
	}
	if err := json.Unmarshal(raw, &enrollment); err != nil {
		t.Fatal(err)
	}
	if enrollment.BundleRoot == "" || enrollment.ManifestSHA256 == "" {
		t.Fatalf("fixture enrollment has no manifest-pinned arguments: %q", input)
	}
	return enrollment.BundleRoot, enrollment.ManifestSHA256
}

func writeEnrollmentFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatal(err)
	}
}

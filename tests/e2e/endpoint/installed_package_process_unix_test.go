//go:build linux && packagee2e

package endpoint_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes proves the installed
// Endpoint package shape with a real command artifact. It runs dpkg against a
// test-owned package database/image root instead of modifying the host. The
// selected Linux package-e2e profile executes this test as root because dpkg
// and setpriv must create and verify root-owned package files before the
// Endpoint runs as an unprivileged user.
//
// The per-user Portable enrollment route is retired (ADR-0125, #617 S3.5): the
// installed `enroll-installed` invocation must refuse before input or root
// effects, creating no Release floor or live root. The package contract this
// profile retains is that install/upgrade/remove/purge keep user state outside
// package ownership; a test-local user-state marker stands in for the retained
// data the retired enrollment formerly committed.
func TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes(t *testing.T) {
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Fatal("installed Endpoint package process profile requires dpkg-deb")
	}
	if _, err := exec.LookPath("dpkg"); err != nil {
		t.Fatal("installed Endpoint package process profile requires dpkg")
	}
	if _, err := exec.LookPath("setpriv"); err != nil {
		t.Fatal("installed Endpoint package process profile requires setpriv")
	}
	command := buildArdents(t)
	bundle, enrolled, alphaInput := packageEnrollmentBundle(t, command)
	staticRoot := packageStaticEnrollment(t, bundle, filepath.Base(enrolled))
	imageRoot, err := os.MkdirTemp("/tmp", "package-image-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeEndpointProcessTree(t, imageRoot) })
	packagePath := filepath.Join(t.TempDir(), "ardents.deb")
	builder := exec.Command("sh", packageBuilder(t))
	builder.Env = append(os.Environ(), "ARDENTS_PACKAGE_VERSION=0.0.0~alpha1", "ARDENTS_PACKAGE_PROGRAM="+enrolled,
		"ARDENTS_PACKAGE_STATIC_ROOT="+staticRoot, "ARDENTS_PACKAGE_OUTPUT="+packagePath)
	if output, err := builder.CombinedOutput(); err != nil {
		t.Fatalf("build Ubuntu package: %v\n%s", err, output)
	}
	installUbuntuPackage(t, packagePath, imageRoot)
	if err := os.Chmod(imageRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(imageRoot, "usr", "lib", "ardents", "ardents")
	launcher := filepath.Join(imageRoot, "usr", "bin", "ardents")
	staticEnrollment := installedStaticEnrollmentRoot(imageRoot, "0.0.0~alpha1")
	for _, path := range []string{installed, launcher, filepath.Join(staticEnrollment, "SHA256SUMS"), filepath.Join(staticEnrollment, "RELEASE")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("package payload %s: %v", path, err)
		}
	}
	if launcherBytes, err := os.ReadFile(launcher); err != nil || string(launcherBytes) != "#!/bin/sh\nexec /usr/lib/ardents/ardents \"$@\"\n" {
		t.Fatalf("package launcher = %q, %v", launcherBytes, err)
	}
	packageInput := installedEnrollmentInput(t, alphaInput, staticEnrollment, installed)
	stateRoot, err := os.MkdirTemp("/tmp", "ip-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeEndpointProcessTree(t, stateRoot) })
	if err := os.Chown(stateRoot, 1000, 1000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(packageInput, 1000, 1000); err != nil {
		t.Fatal(err)
	}
	environment := endpointEnvironment(stateRoot)
	assertInstalledEnrollmentRetired(t, installed, packageInput, environment)
	// The retired enrollment creates no state. Seed one user-owned marker
	// outside package ownership to prove the package lifecycle retains it.
	userState := filepath.Join(stateRoot, "state", "ardents", "live")
	if err := os.MkdirAll(userState, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(userState, "retained-user-state")
	if err := os.WriteFile(marker, []byte("retained user state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(userState, 1000, 1000); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(marker, 1000, 1000); err != nil {
		t.Fatal(err)
	}

	v1, err := os.ReadFile(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	v2 := append(append([]byte(nil), v1...), 0)
	v2Program := filepath.Join(t.TempDir(), "ardents-v2")
	if err := os.WriteFile(v2Program, v2, 0o700); err != nil {
		t.Fatal(err)
	}
	v2Package := filepath.Join(t.TempDir(), "ardents-v2.deb")
	v2Builder := exec.Command("sh", packageBuilder(t))
	v2Builder.Env = append(os.Environ(), "ARDENTS_PACKAGE_VERSION=0.0.0~alpha2", "ARDENTS_PACKAGE_PROGRAM="+v2Program,
		"ARDENTS_PACKAGE_STATIC_ROOT="+staticRoot, "ARDENTS_PACKAGE_OUTPUT="+v2Package)
	if output, err := v2Builder.CombinedOutput(); err != nil {
		t.Fatalf("build upgraded Ubuntu package: %v\n%s", err, output)
	}
	installUbuntuPackage(t, v2Package, imageRoot)
	if upgraded, readErr := os.ReadFile(installed); readErr != nil || !bytes.Equal(upgraded, v2) {
		t.Fatalf("upgraded package program = %d bytes / %v", len(upgraded), readErr)
	}
	assertRetainedUserState(t, marker)
	removeUbuntuPackage(t, imageRoot)
	if _, err := os.Stat(filepath.Join(imageRoot, "usr", "lib", "ardents", "ardents")); !os.IsNotExist(err) {
		t.Fatalf("dpkg remove retained package program: %v", err)
	}
	assertRetainedUserState(t, marker)
	purgeUbuntuPackage(t, imageRoot)
	assertRetainedUserState(t, marker)
}

// assertInstalledEnrollmentRetired runs the installed program as the
// unprivileged service user and requires the typed retirement refusal before
// any lifecycle output or state effect.
func assertInstalledEnrollmentRetired(t *testing.T, command, input string, environment []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	arguments := []string{"--reuid=1000", "--regid=1000", "--clear-groups", command, "endpoint", "enroll-installed", input}
	running := exec.CommandContext(ctx, "setpriv", arguments...)
	running.Env = environment
	var stdout, stderr bytes.Buffer
	running.Stdout, running.Stderr = &stdout, &stderr
	err := running.Run()
	if err == nil {
		t.Fatalf("retired enroll-installed unexpectedly succeeded: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "endpoint enroll-installed is retired") {
		t.Fatalf("retired enroll-installed refusal = %v; stderr=%s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("retired enroll-installed emitted lifecycle output: %s", stdout.String())
	}
}

func assertRetainedUserState(t *testing.T, marker string) {
	t.Helper()
	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "retained user state\n" {
		t.Fatalf("package lifecycle erased retained user state: %q / %v", raw, err)
	}
}

// packageEnrollmentBundle assembles one ephemeral closed-alpha bundle around
// the exact command artifact for the package payload shape. It reuses the
// shared pinned Release-metadata fixtures; the retired enrollment never
// verifies this bundle, so its inert companion payloads remain pinned facts.
func packageEnrollmentBundle(t *testing.T, command string) (string, string, string) {
	t.Helper()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	artifactName := "ardents-" + platform
	controlName := "ardents-control-" + platform
	nodeName := "ardents-node-" + platform
	custodyName := "ardents-custody-" + platform
	artifact, err := os.ReadFile(command)
	if err != nil {
		t.Fatal(err)
	}
	targetPath := "ardents/" + platform + "/endpoint"
	now := time.Now().UTC().Truncate(time.Second)
	metadataFiles, rootBytes := enrolledRuntimeMetadata(t, artifact, targetPath, platform, now)
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v3", "cohort=closed-cohort-1", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json", "control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1", "control_compatibility=compatibility.ac1", "control_release_root=release.pub", "control_network_root=network.pub", "control_compatibility_root=compatibility.pub", "corpus_authority=corpus.pub", "control_artifact=" + controlName,
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact, controlName: []byte("control"), nodeName: []byte("node"), custodyName: []byte("custody"), "catalog.ac1": []byte("catalog"), "catalog.pub": []byte("key"), "release.ac1": []byte("release control"), "network.ac1": []byte("network control"), "compatibility.ac1": []byte("compatibility control"), "release.pub": []byte("release key"), "network.pub": []byte("network key"), "compatibility.pub": []byte("compatibility key"), "corpus.pub": []byte("corpus")}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	names := []string{"1.root.json", "1.snapshot.json", "1.targets.json", "RELEASE", artifactName, controlName, nodeName, custodyName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "corpus.pub", "timestamp.json"}
	for _, name := range names {
		mode := os.FileMode(0o600)
		if name == artifactName {
			mode = 0o700
		}
		writeEnrollmentFile(t, filepath.Join(bundle, name), files[name], mode)
	}
	manifest := enrolledRuntimeManifest(t, names, files)
	writeEnrollmentFile(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	pin := sha256.Sum256(manifest)
	input := filepath.Join(t.TempDir(), "alpha-enrollment.json")
	raw, err := json.Marshal(map[string]string{
		"schema": "ardents-alpha-enrollment-input-v1", "bundle_root": bundle, "cohort": "closed-cohort-1", "release": "alpha-1",
		"platform": platform, "manifest_sha256": hex.EncodeToString(pin[:]), "environment": "alpha", "network": "alpha-network-1", "target_path": targetPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, input, raw, 0o600)
	return bundle, filepath.Join(bundle, artifactName), input
}

func endpointEnvironment(root string) []string {
	return append(os.Environ(), "XDG_STATE_HOME="+filepath.Join(root, "state"),
		"XDG_RUNTIME_DIR="+filepath.Join(root, "runtime"))
}

func removeEndpointProcessTree(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Errorf("remove process fixture %s: %v", path, err)
		return
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("process fixture remains at %s: %v", path, err)
	}
}

func installedStaticEnrollmentRoot(imageRoot, version string) string {
	return filepath.Join(imageRoot, "usr", "share", "ardents", "enrollment", version)
}

func packageStaticEnrollment(t *testing.T, bundle, artifactName string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "enrollment")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == artifactName {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(bundle, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, entry.Name()), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func packageBuilder(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate package process test")
	}
	return filepath.Join(filepath.Dir(source), "..", "..", "..", "packaging", "ubuntu-deb", "build.sh")
}

func installUbuntuPackage(t *testing.T, packagePath, imageRoot string) {
	t.Helper()
	admin := filepath.Join(imageRoot, "var", "lib", "dpkg")
	if err := os.MkdirAll(admin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(admin, "status"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("dpkg", "--root="+imageRoot, "--admindir="+admin, "--instdir="+imageRoot, "--install", packagePath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("install Ubuntu package: %v\n%s", err, output)
	}
}

func removeUbuntuPackage(t *testing.T, imageRoot string) {
	t.Helper()
	admin := filepath.Join(imageRoot, "var", "lib", "dpkg")
	command := exec.Command("dpkg", "--root="+imageRoot, "--admindir="+admin, "--instdir="+imageRoot, "--remove", "ardents")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("remove Ubuntu package: %v\n%s", err, output)
	}
}

func purgeUbuntuPackage(t *testing.T, imageRoot string) {
	t.Helper()
	admin := filepath.Join(imageRoot, "var", "lib", "dpkg")
	command := exec.Command("dpkg", "--root="+imageRoot, "--admindir="+admin, "--instdir="+imageRoot, "--purge", "ardents")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("purge Ubuntu package: %v\n%s", err, output)
	}
}

func installedEnrollmentInput(t *testing.T, alphaInput, staticRoot, artifactPath string) string {
	t.Helper()
	var alpha struct {
		Cohort         string `json:"cohort"`
		Release        string `json:"release"`
		Platform       string `json:"platform"`
		ManifestSHA256 string `json:"manifest_sha256"`
		Environment    string `json:"environment"`
		Network        string `json:"network"`
		TargetPath     string `json:"target_path"`
	}
	raw, err := os.ReadFile(alphaInput)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &alpha); err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(map[string]string{"schema": "ardents-ubuntu-package-enrollment-input-v1", "static_root": staticRoot,
		"artifact_path": artifactPath, "cohort": alpha.Cohort, "release": alpha.Release, "platform": alpha.Platform,
		"manifest_sha256": alpha.ManifestSHA256, "environment": alpha.Environment, "network": alpha.Network, "target_path": alpha.TargetPath})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp("/tmp", "package-enrollment-*.json")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("remove package enrollment input %s: %v", path, err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("package enrollment input remains at %s: %v", path, err)
		}
	})
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

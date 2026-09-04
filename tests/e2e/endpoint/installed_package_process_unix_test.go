//go:build linux

package endpoint_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes proves the first
// installed Endpoint package shape with a real command artifact. It runs dpkg
// against a test-owned package database/image root instead of modifying the
// host. The selected Linux profile executes this test as root because dpkg and
// setpriv must create and verify root-owned package files before the Endpoint
// runs as an unprivileged user.
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
	bundle, enrolled, alphaInput, keys, rootBytes := enrolledRuntimeBundleWithKeys(t, command)
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
	runInstalledUntilStopped(t, installed, packageInput, environment)

	v1, err := os.ReadFile(enrolled)
	if err != nil {
		t.Fatal(err)
	}
	v2 := append(append([]byte(nil), v1...), 0)
	v2Program := filepath.Join(t.TempDir(), "ardents-v2")
	if err := os.WriteFile(v2Program, v2, 0o700); err != nil {
		t.Fatal(err)
	}
	v2Bundle, v2Input := upgradedPackageEnrollmentBundle(t, v2, rootBytes, keys)
	v2StaticRoot := packageStaticEnrollment(t, v2Bundle, "ardents-"+runtime.GOOS+"-"+runtime.GOARCH)
	v2Package := filepath.Join(t.TempDir(), "ardents-v2.deb")
	v2Builder := exec.Command("sh", packageBuilder(t))
	v2Builder.Env = append(os.Environ(), "ARDENTS_PACKAGE_VERSION=0.0.0~alpha2", "ARDENTS_PACKAGE_PROGRAM="+v2Program,
		"ARDENTS_PACKAGE_STATIC_ROOT="+v2StaticRoot, "ARDENTS_PACKAGE_OUTPUT="+v2Package)
	if output, err := v2Builder.CombinedOutput(); err != nil {
		t.Fatalf("build upgraded Ubuntu package: %v\n%s", err, output)
	}
	installUbuntuPackage(t, v2Package, imageRoot)
	v2InstalledInput := installedEnrollmentInput(t, v2Input, installedStaticEnrollmentRoot(imageRoot, "0.0.0~alpha2"), installed)
	if err := os.Chown(v2InstalledInput, 1000, 1000); err != nil {
		t.Fatal(err)
	}
	runInstalledUntilStopped(t, installed, v2InstalledInput, environment)
	if upgraded, readErr := os.ReadFile(installed); readErr != nil || !bytes.Equal(upgraded, v2) {
		t.Fatalf("upgraded package program = %d bytes / %v", len(upgraded), readErr)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "state", "ardents", "vault")); err != nil {
		t.Fatalf("package upgrade erased Vault root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "state", "ardents", "floors", "release-decision", "current")); err != nil {
		t.Fatalf("package upgrade erased Release floor: %v", err)
	}
	removeUbuntuPackage(t, imageRoot)
	if _, err := os.Stat(filepath.Join(imageRoot, "usr", "lib", "ardents", "ardents")); !os.IsNotExist(err) {
		t.Fatalf("dpkg remove retained package program: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "state", "ardents", "vault")); err != nil {
		t.Fatalf("package removal erased Vault root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "state", "ardents", "floors", "release-decision", "current")); err != nil {
		t.Fatalf("package removal erased Release floor: %v", err)
	}
	purgeUbuntuPackage(t, imageRoot)
	if _, err := os.Stat(filepath.Join(stateRoot, "state", "ardents", "vault")); err != nil {
		t.Fatalf("package purge erased Vault root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "state", "ardents", "floors", "release-decision", "current")); err != nil {
		t.Fatalf("package purge erased Release floor: %v", err)
	}
}

func installedStaticEnrollmentRoot(imageRoot, version string) string {
	return filepath.Join(imageRoot, "usr", "share", "ardents", "enrollment", version)
}

func upgradedPackageEnrollmentBundle(t *testing.T, artifact, rootBytes []byte, keys []enrolledRuntimeKey) (string, string) {
	t.Helper()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	targetPath := "ardents/" + platform + "/endpoint"
	metadataFiles := replacementMetadataVersion(t, artifact, targetPath, platform, time.Now().UTC().Truncate(time.Second), keys, 2, 2)
	metadataFiles["2.root.json"] = replacementRootVersion(t, rootBytes, keys, 2)
	artifactName := "ardents-" + platform
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v1", "cohort=closed-cohort-1", "release=alpha-2", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json", "control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1", "control_compatibility=compatibility.ac1", "control_release_root=release.pub", "control_network_root=network.pub", "control_compatibility_root=compatibility.pub",
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact, "catalog.ac1": []byte("catalog"), "catalog.pub": []byte("key"), "release.ac1": []byte("release control"), "network.ac1": []byte("network control"), "compatibility.ac1": []byte("compatibility control"), "release.pub": []byte("release key"), "network.pub": []byte("network key"), "compatibility.pub": []byte("compatibility key")}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	names := []string{"1.root.json", "2.root.json", "2.snapshot.json", "2.targets.json", "RELEASE", artifactName, "catalog.ac1", "catalog.pub", "compatibility.ac1", "compatibility.pub", "network.ac1", "network.pub", "release.ac1", "release.pub", "timestamp.json"}
	bundle := filepath.Join(t.TempDir(), "bundle-v2")
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
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
	input := filepath.Join(t.TempDir(), "alpha-enrollment-v2.json")
	raw, err := json.Marshal(map[string]string{"schema": "ardents-alpha-enrollment-input-v1", "bundle_root": bundle, "cohort": "closed-cohort-1", "release": "alpha-2",
		"platform": platform, "manifest_sha256": hex.EncodeToString(pin[:]), "environment": "alpha", "network": "alpha-network-1", "target_path": targetPath})
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, input, raw, 0o600)
	return bundle, input
}

func runInstalledUntilStopped(t *testing.T, command, input string, environment []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	arguments := []string{"--reuid=1000", "--regid=1000", "--clear-groups", command, "endpoint", "enroll-installed", input}
	running := exec.CommandContext(ctx, "setpriv", arguments...)
	running.Env = environment
	stdout, err := running.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := &processStderrBuffer{}
	running.Stderr = stderr
	if err := running.Start(); err != nil {
		t.Fatal(err)
	}
	finished := false
	t.Cleanup(func() {
		if finished {
			return
		}
		if err := running.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("terminate installed Endpoint after failed process assertion: %v", err)
		}
		if err := running.Wait(); err != nil {
			t.Errorf("join installed Endpoint after failed process assertion: %v", err)
		}
	})
	scanner := bufio.NewScanner(stdout)
	ready := false
	for scanner.Scan() {
		var event struct{ Kind, State string }
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Kind == "endpoint-lifecycle" && event.State == "ready" {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("Installed Endpoint did not become ready: %v; %s", scanner.Err(), stderr.String())
	}
	if err := running.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatalf("Installed Endpoint did not report stopped: %v; %s", scanner.Err(), stderr.String())
	}
	waitErr := running.Wait()
	finished = true
	if waitErr != nil {
		t.Fatalf("Installed Endpoint exit: %v; %s", waitErr, stderr.String())
	}
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

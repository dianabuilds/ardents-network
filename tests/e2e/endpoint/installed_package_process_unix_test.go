//go:build linux && packagee2e

package endpoint_test

import (
	"bytes"
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
// selected Linux package-e2e profile executes this test as root to verify the
// root-owned package files and preservation of user-owned state.
func TestUbuntuDebInstallsOnlyProgramAndStaticEnrollmentBytes(t *testing.T) {
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Fatal("installed Endpoint package process profile requires dpkg-deb")
	}
	if _, err := exec.LookPath("dpkg"); err != nil {
		t.Fatal("installed Endpoint package process profile requires dpkg")
	}
	command := buildArdentsNext(t)
	bundle, enrolled := packageEnrollmentBundle(t, command)
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
	foreign := filepath.Join(staticRoot, "ardents-custody-linux-amd64")
	writeEnrollmentFile(t, foreign, []byte("foreign Custody distribution"), 0o600)
	rejected := filepath.Join(t.TempDir(), "rejected.deb")
	rejectBuilder := exec.Command("sh", packageBuilder(t))
	rejectBuilder.Env = append(append([]string(nil), builder.Env...), "ARDENTS_PACKAGE_OUTPUT="+rejected)
	if output, err := rejectBuilder.CombinedOutput(); err == nil || !strings.Contains(string(output), "invalid enrollment inventory") {
		t.Fatalf("package builder accepted unexpected Custody payload: %v\n%s", err, output)
	}
	if _, err := os.Stat(rejected); !os.IsNotExist(err) {
		t.Fatalf("refused build created a package: %v", err)
	}
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
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
	stateRoot, err := os.MkdirTemp("/tmp", "ip-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeEndpointProcessTree(t, stateRoot) })
	if err := os.Chown(stateRoot, 1000, 1000); err != nil {
		t.Fatal(err)
	}
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
	v2Bundle, v2Enrolled := packageEnrollmentBundle(t, v2Program)
	v2Static := packageStaticEnrollment(t, v2Bundle, filepath.Base(v2Enrolled))
	v2Package := filepath.Join(t.TempDir(), "ardents-v2.deb")
	v2Builder := exec.Command("sh", packageBuilder(t))
	v2Builder.Env = append(os.Environ(), "ARDENTS_PACKAGE_VERSION=0.0.0~alpha2", "ARDENTS_PACKAGE_PROGRAM="+v2Program,
		"ARDENTS_PACKAGE_STATIC_ROOT="+v2Static, "ARDENTS_PACKAGE_OUTPUT="+v2Package)
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

func assertRetainedUserState(t *testing.T, marker string) {
	t.Helper()
	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "retained user state\n" {
		t.Fatalf("package lifecycle erased retained user state: %q / %v", raw, err)
	}
}

// packageEnrollmentBundle assembles one ephemeral closed-alpha bundle around
// the exact command artifact to exercise the package payload shape.
func packageEnrollmentBundle(t *testing.T, command string) (string, string) {
	t.Helper()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	artifactName := "ardents-" + platform
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
		"schema=ardents-closed-alpha-enrollment-v6", "cohort=closed-cohort-1", "release=alpha-1", "platform=" + platform,
		"environment=alpha", "network=alpha-network-1", "target_path=" + targetPath, "artifact=" + artifactName, "trusted_root=1.root.json",
	}, "\n") + "\n"
	files := map[string][]byte{"1.root.json": rootBytes, "RELEASE": []byte(descriptor), artifactName: artifact}
	for name, contents := range metadataFiles {
		files[name] = contents
	}
	names := []string{"1.root.json", "1.snapshot.json", "1.targets.json", "RELEASE", artifactName, "timestamp.json"}
	for _, name := range names {
		mode := os.FileMode(0o600)
		if name == artifactName {
			mode = 0o700
		}
		writeEnrollmentFile(t, filepath.Join(bundle, name), files[name], mode)
	}
	manifest := enrolledRuntimeManifest(t, names, files)
	writeEnrollmentFile(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	return bundle, filepath.Join(bundle, artifactName)
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

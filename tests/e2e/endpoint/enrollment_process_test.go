package endpoint_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestHeadlessEnrollmentAcceptsExactRunningBundleAndRejectsChangedManifest(t *testing.T) {
	command := buildArdentsNext(t)
	bundle, enrolledCommand, manifestPin := enrollmentBundle(t, command)

	artifact, err := os.Stat(enrolledCommand)
	if err != nil {
		t.Fatal(err)
	}
	check := func(pin, want string, success bool) {
		t.Helper()
		command := exec.CommandContext(t.Context(), enrolledCommand, "enrollment", "verify", bundle, pin)
		output, err := command.CombinedOutput()
		if (err == nil) != success || string(output) != want {
			t.Fatalf("headless enrollment result: err=%v output=%q", err, output)
		}
	}
	check(manifestPin, fmt.Sprintf("{\"outcome\":\"verified-initial-bundle\",\"files\":6,\"program_bytes\":%d}\n", artifact.Size()), true)
	check(strings.Repeat("0", 64), "{\"outcome\":\"pin-mismatch\"}\n", false)

	manifestPath := filepath.Join(bundle, "SHA256SUMS")
	original, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	writeEnrollmentFile(t, manifestPath, []byte("changed-before-parse\n"), 0o600)
	check(manifestPin, "{\"outcome\":\"pin-mismatch\"}\n", false)
	writeEnrollmentFile(t, manifestPath, original, 0o600)
	writeEnrollmentFile(t, filepath.Join(bundle, "timestamp.json"), []byte("substituted"), 0o600)
	check(manifestPin, "{\"outcome\":\"binding-mismatch\"}\n", false)
}

func buildArdentsNext(t *testing.T) string {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_E2E_COMMAND"); prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt Ardents Next command is not a regular file: %v", err)
		}
		return prebuilt
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E source")
	}
	name := "ardents-next"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", path, "./cmd/ardents-next")
	build.Dir = filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build Ardents Next command: %v\n%s", err, output)
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
		"schema=ardents-closed-alpha-enrollment-v6",
		"cohort=closed-cohort-1",
		"release=alpha-1",
		"platform=" + platform,
		"environment=alpha",
		"network=alpha-network-1",
		"target_path=" + targetPath,
		"artifact=" + artifactName,
		"trusted_root=1.root.json",
	}, "\n") + "\n"
	files := map[string][]byte{
		"1.root.json":     []byte("synthetic trusted root\n"),
		"RELEASE":         []byte(descriptor),
		artifactName:      artifact,
		"timestamp.json":  []byte("synthetic metadata\n"),
		"1.snapshot.json": []byte("synthetic snapshot\n"),
		"1.targets.json":  []byte("synthetic targets\n"),
	}
	for name, contents := range files {
		if name != artifactName {
			writeEnrollmentFile(t, filepath.Join(bundle, name), contents, 0o600)
		}
	}
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
	manifest := []byte(strings.Join(lines, "\n") + "\n")
	writeEnrollmentFile(t, filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600)
	pinned := sha256.Sum256(manifest)
	return bundle, filepath.Join(bundle, artifactName), hex.EncodeToString(pinned[:])
}

func writeEnrollmentFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatal(err)
	}
}

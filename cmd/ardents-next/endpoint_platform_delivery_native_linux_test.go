//go:build linux && (endpoint_launch_native || endpoint_pair_native)

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	requestinput "github.com/dianabuilds/ardents-network/internal/endpoint/plan"
)

func platformNativeDistribution(t *testing.T, src, dst string) (string, string) {
	t.Helper()
	if err := os.Mkdir(dst, 0700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	return dst, filepath.Join(dst, "ardents-linux-amd64")
}

func platformNativeLaunch(t *testing.T, requestPath, distribution string) platformLaunch {
	t.Helper()
	raw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	decl, err := requestinput.DecodeDeclaration(raw)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := requestinput.PlatformWritableRoots(decl)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := requestinput.PlatformSourceCredentials(decl)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make(map[string]string, len(credentials))
	for _, path := range credentials {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		inputs[path] = platformDigest(data)
	}
	descriptor, err := os.ReadFile(filepath.Join(distribution, "protected-endpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	return platformLaunch{
		Role: "endpoint", Distribution: distribution, ReleaseRoot: decl.ReleaseFloorRoot,
		Environment: "h3-test", Network: "new-release-test",
		Program:       filepath.Join(decl.InstallationRoot, "generations", platformDigest(descriptor), "ardents-linux-amd64"),
		Configuration: requestPath, ConfigSHA256: platformDigest(raw), WritableRoots: roots, Inputs: inputs,
	}
}

func writePlatformNativeLaunch(t *testing.T, path string, launch platformLaunch) string {
	t.Helper()
	raw, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	writePlatformNativeFile(t, path, raw, 0644)
	return platformDigest(raw)
}

func writePlatformNativeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// The platform prepares the account, roots and fixed Worker files through the
// host. Ardents authenticates the bundle and seals the compatible generation.
func stagePlatformNativeEndpoint(t *testing.T, requestPath, initialProgram, pin, distribution, keyPath string) (uint32, uint32) {
	t.Helper()
	launch := platformNativeLaunch(t, requestPath, filepath.Dir(initialProgram))
	var id int
	for n := 990; n >= 100; n-- {
		candidate := strconv.Itoa(n)
		if exec.Command("getent", "passwd", candidate).Run() != nil && exec.Command("getent", "group", candidate).Run() != nil {
			id = n
			break
		}
	}
	if id == 0 {
		t.Fatal("no vacant Endpoint UID/GID")
	}
	numeric := strconv.Itoa(id)
	for _, args := range [][]string{
		{"groupadd", "-g", numeric, "ardents-endpoint"},
		{"useradd", "-u", numeric, "-g", numeric, "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-endpoint"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v: %s", args[0], err, out)
		}
	}
	for _, path := range launch.WritableRoots {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(path, id, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chown(keyPath, id, id); err != nil {
		t.Fatal(err)
	}
	initialManifest := filepath.Join(filepath.Dir(requestPath), "initialize.json")
	writePlatformNativeLaunch(t, initialManifest, launch)
	for _, args := range [][]string{
		{"platform", "initialize-trust", initialManifest, pin},
		{"platform", "stage-endpoint", requestPath},
	} {
		if out, err := exec.Command(initialProgram, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v: %s", args[1], err, out)
		}
	}
	platformNativeWorkers(t, distribution, id)
	return uint32(id), uint32(id)
}

func platformNativeWorkers(t *testing.T, distribution string, gid int) {
	t.Helper()
	const workerRoot = "/usr/lib/ardents/text-worker-root"
	if err := os.MkdirAll(filepath.Dir(workerRoot), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workerRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dev", "proc", "sys", "run", "tmp", "etc", "root", "usr", "var", "var/tmp"} {
		if err := os.Mkdir(filepath.Join(workerRoot, name), 0555); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		workerRoot + "/ardents-text":                          "ardents-text-linux-amd64",
		"/etc/systemd/system/ardents-text-reader@.service":    "ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service": "ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket":      "ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket":   "ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules":   "50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf":               "ardents-text.conf",
	}
	digests := make(map[string]string)
	for path, name := range files {
		data, err := os.ReadFile(filepath.Join(distribution, name))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0644)
		if name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		writePlatformNativeFile(t, path, data, mode)
		if name != "ardents-text.conf" {
			digests[path] = platformDigest(data)
		}
	}
	if err := os.Chmod(workerRoot, 0555); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
	if err != nil {
		t.Fatal(err)
	}
	writePlatformNativeFile(t, "/etc/ardents/text-worker-artifact.json", append(manifest, '\n'), 0644)
	if err := os.Mkdir("/run/ardents-text", 0710); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown("/run/ardents-text", 0, gid); err != nil {
		t.Fatal(err)
	}
}

func installPlatformNativeUnit(t *testing.T, requestPath, distribution, program string) (string, string) {
	t.Helper()
	launch := platformNativeLaunch(t, requestPath, distribution)
	if launch.Program != program {
		t.Fatal("platform unit and staged generation differ")
	}
	manifest := filepath.Join(filepath.Dir(requestPath), "launch.json")
	pin := writePlatformNativeLaunch(t, manifest, launch)
	unit, err := exec.Command(filepath.Join(distribution, "ardents-linux-amd64"), "platform", "unit", manifest).CombinedOutput()
	if err != nil {
		t.Fatalf("platform unit: %v: %s", err, unit)
	}
	writePlatformNativeFile(t, "/etc/systemd/system/ardents-endpoint.service", unit, 0644)
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		t.Fatal(fmt.Errorf("daemon-reload: %w: %s", err, out))
	}
	return manifest, pin
}

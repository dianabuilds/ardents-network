package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
	"github.com/dianabuilds/ardents-network/internal/browser/entry/enrollment"
	"github.com/dianabuilds/ardents-network/internal/browser/entry/installer"
)

func TestParticipantInstallAuthenticatesARealV4Bundle(t *testing.T) {
	bundle := t.TempDir()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	endpointName := "ardents-" + platform
	hostName := browserentry.HostArtifactName(platform)
	adapterName := "ardents-browser-" + platform
	if runtime.GOOS == "windows" {
		adapterName += ".exe"
	}
	controlName := "ardents-control-" + platform
	for _, name := range []string{endpointName, adapterName, hostName, controlName} {
		copyEnrollmentArtifact(t, executable, filepath.Join(bundle, name))
	}
	extension := writeBrowserEntryExtension(t, bundle)
	descriptor := strings.Join([]string{
		"schema=ardents-closed-alpha-enrollment-v4",
		"cohort=browser-entry-test",
		"release=browser-entry-0.1.0",
		"platform=" + platform,
		"environment=alpha",
		"network=browser-entry-test-network",
		"target_path=ardents/" + platform + "/endpoint",
		"artifact=" + endpointName,
		"trusted_root=1.root.json",
		"control_catalog=catalog.ac1",
		"disclosure_root=catalog.pub",
		"control_release=release.ac1",
		"control_network=network.ac1",
		"control_compatibility=compatibility.ac1",
		"control_release_root=release.pub",
		"control_network_root=network.pub",
		"control_compatibility_root=compatibility.pub",
		"corpus_authority=corpus.pub",
		"control_artifact=" + controlName,
		"browser_adapter_artifact=" + adapterName,
		"browser_entry_artifact=" + hostName,
		"browser_entry_extension=" + browserentry.ExtensionArtifactName,
	}, "\n") + "\n"
	static := map[string][]byte{
		"1.root.json":       []byte("test trusted root"),
		"RELEASE":           []byte(descriptor),
		"catalog.ac1":       []byte("test control catalog"),
		"catalog.pub":       []byte("test disclosure root"),
		"compatibility.ac1": []byte("test compatibility decision"),
		"compatibility.pub": []byte("test compatibility root"),
		"corpus.pub":        []byte("test corpus authority"),
		"network.ac1":       []byte("test network decision"),
		"network.pub":       []byte("test network root"),
		"release.ac1":       []byte("test release decision"),
		"release.pub":       []byte("test release root"),
		"timestamp.json":    []byte("test timestamp"),
	}
	for name, contents := range static {
		if err := os.WriteFile(filepath.Join(bundle, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := make(map[string][]byte, len(static)+4)
	for name, contents := range static {
		files[name] = contents
	}
	for _, name := range []string{endpointName, adapterName, hostName, controlName, browserentry.ExtensionArtifactName} {
		contents, readErr := os.ReadFile(filepath.Join(bundle, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		files[name] = contents
	}
	manifest := browserEntryEnrollmentManifest(files)
	if err := os.WriteFile(filepath.Join(bundle, "SHA256SUMS"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pinned := sha256.Sum256(manifest)
	inputPath := filepath.Join(t.TempDir(), "enrollment.json")
	input := enrollment.ClosedAlphaInput{Schema: "ardents-alpha-enrollment-input-v1", BundleRoot: bundle,
		Cohort: "browser-entry-test", Release: "browser-entry-0.1.0", Platform: platform,
		ManifestSHA256: hex.EncodeToString(pinned[:]), Environment: "alpha", Network: "browser-entry-test-network",
		TargetPath: "ardents/" + platform + "/endpoint"}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	var installedHost, installedExtension string
	verifiedCompanion := false
	var output strings.Builder
	err = installBrowserEntryWith([]string{"--enrollment", inputPath, "--endpoint-artifact", filepath.Join(bundle, endpointName), "--at", time.Now().UTC().Format(time.RFC3339)}, &output,
		enrollment.Verify, func(_ enrollment.Request, name string, artifact []byte) error {
			verifiedCompanion = name == hostName
			actual, readErr := os.ReadFile(filepath.Join(bundle, hostName))
			if readErr != nil || !bytes.Equal(artifact, actual) {
				t.Fatalf("Browser Entry command did not pass its verified host companion: read=%v", readErr)
			}
			return nil
		}, func(host, extensionPath string) (installer.Result, error) {
			installedHost, installedExtension = host, extensionPath
			return installer.Result{NativeManifestPath: filepath.Join(bundle, "native-manifest.json"), ExtensionPath: extensionPath}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !verifiedCompanion || installedHost != filepath.Join(bundle, hostName) || installedExtension != extension || !strings.Contains(output.String(), `"extension_installation":"manual-required"`) {
		t.Fatalf("participant Browser Entry installation did not retain its exact v4 companions: host=%q extension=%q output=%q", installedHost, installedExtension, output.String())
	}
}

func copyEnrollmentArtifact(t *testing.T, source, target string) {
	t.Helper()
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, contents, 0o700); err != nil {
		t.Fatalf("write enrollment artifact %q: %v", target, err)
	}
}

func writeBrowserEntryExtension(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, browserentry.ExtensionArtifactName)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, contents := range map[string][]byte{
		"manifest.json":          []byte(`{"browser_specific_settings":{"gecko":{"id":"alpha-browser-entry@ardents.network"}}}`),
		"META-INF/cose.manifest": []byte("Mozilla signature manifest"),
		"META-INF/cose.sig":      []byte("Mozilla signature"),
	} {
		entry, entryErr := archive.Create(name)
		if entryErr != nil {
			t.Fatal(entryErr)
		}
		if _, entryErr := entry.Write(contents); entryErr != nil {
			t.Fatal(entryErr)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func browserEntryEnrollmentManifest(files map[string][]byte) []byte {
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

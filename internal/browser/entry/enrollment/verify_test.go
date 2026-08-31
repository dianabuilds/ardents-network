package enrollment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestVerifyReturnsOnlyAuthenticatedBrowserCompanions(t *testing.T) {
	t.Parallel()
	request, companions := writeEnrollmentFixture(t)

	verified, err := Verify(request)
	if err != nil {
		t.Fatal(err)
	}
	if verified.BrowserAdapterArtifactName != companions[0] || verified.BrowserEntryArtifactName != companions[1] ||
		verified.BrowserEntryExtensionName != companions[2] || string(verified.BrowserAdapterArtifact) != "adapter" ||
		string(verified.BrowserEntryArtifact) != "entry" || string(verified.BrowserEntryExtension) != "extension" {
		t.Fatalf("verified Browser companions = %+v", verified)
	}
	if err := os.WriteFile(filepath.Join(request.BundleRoot, companions[0]), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(request); err == nil {
		t.Fatal("Verify accepted a Browser companion changed after enrollment")
	}
}

func TestReadClosedAlphaInputRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	request, _ := writeEnrollmentFixture(t)
	input := ClosedAlphaInput{Schema: inputSchema, BundleRoot: request.BundleRoot, Cohort: request.Pin.Cohort,
		Release: request.Pin.Release, Platform: request.Pin.Platform, ManifestSHA256: request.Pin.ManifestSHA256,
		Environment: request.Environment, Network: request.Network, TargetPath: request.TargetPath}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "enrollment.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := ReadClosedAlphaInput(path)
	if err != nil || read != input {
		t.Fatalf("ReadClosedAlphaInput() = (%+v, %v)", read, err)
	}
	unknown := append(raw[:len(raw)-1], []byte(`,"unexpected":true}`)...)
	if err := os.WriteFile(path, unknown, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClosedAlphaInput(path); err == nil {
		t.Fatal("ReadClosedAlphaInput accepted an unknown field")
	}
}

func TestVerifyRunningCompanionRejectsIncompleteRequest(t *testing.T) {
	t.Parallel()
	if err := VerifyRunningCompanion(Request{}, "ardents-browser-entry", []byte("entry")); err == nil {
		t.Fatal("VerifyRunningCompanion accepted an incomplete request")
	}
}

func writeEnrollmentFixture(t *testing.T) (Request, [3]string) {
	t.Helper()
	root := t.TempDir()
	platform := runtime.GOOS + "-" + runtime.GOARCH
	endpoint := "ardents-" + platform
	adapter := executableName("ardents-browser", platform)
	entry := executableName("ardents-browser-entry", platform)
	extension := "ardents-alpha-browser-entry.xpi"
	control := "ardents-control-" + platform
	descriptor := strings.Join([]string{
		"schema=" + descriptorSchema,
		"cohort=cohort-1",
		"release=alpha-1",
		"platform=" + platform,
		"environment=alpha",
		"network=network-1",
		"target_path=ardents/" + platform + "/endpoint",
		"artifact=" + endpoint,
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
		"control_artifact=" + control,
		"browser_adapter_artifact=" + adapter,
		"browser_entry_artifact=" + entry,
		"browser_entry_extension=" + extension,
	}, "\n") + "\n"
	files := map[string][]byte{
		descriptorName: []byte(descriptor), endpoint: []byte("endpoint"), adapter: []byte("adapter"), entry: []byte("entry"),
		extension: []byte("extension"), control: []byte("control"), "1.root.json": []byte("root"),
		"catalog.ac1": []byte("catalog"), "catalog.pub": []byte("disclosure"), "release.ac1": []byte("release"),
		"network.ac1": []byte("network"), "compatibility.ac1": []byte("compatibility"), "release.pub": []byte("release root"),
		"network.pub": []byte("network root"), "compatibility.pub": []byte("compatibility root"), "corpus.pub": []byte("corpus"),
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
			t.Fatal(err)
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
	if err := os.WriteFile(filepath.Join(root, manifestName), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(manifest)
	return Request{BundleRoot: root, ExecutablePath: filepath.Join(root, endpoint),
		Pin:         Pin{Cohort: "cohort-1", Release: "alpha-1", Platform: platform, ManifestSHA256: hex.EncodeToString(pin[:])},
		Environment: "alpha", Network: "network-1", TargetPath: "ardents/" + platform + "/endpoint"}, [3]string{adapter, entry, extension}
}

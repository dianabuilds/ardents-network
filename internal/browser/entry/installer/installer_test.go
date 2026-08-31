package installer

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
)

func TestInstallAndRemoveOwnOnlyTheFixedNativeManifest(t *testing.T) {
	root := t.TempDir()
	host := writeInstallerFile(t, root, "ardents-browser-entry", []byte("host"))
	extension := writeExtension(t, root)
	manifestPath := filepath.Join(root, "native", "manifest.json")
	registered := ""
	target := location{path: manifestPath, register: func(path string) error { registered = path; return nil }, unregister: func(path string) error {
		if registered != "" && registered != path {
			t.Fatal("installer attempted to remove a foreign registration")
		}
		registered = ""
		return nil
	}}
	result, err := installAt(host, extension, target)
	if err != nil || result.NativeManifestPath != manifestPath || result.ExtensionPath != extension || registered != manifestPath {
		t.Fatalf("Browser Entry installation = %+v, registration = %q, error = %v", result, registered, err)
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil || !ownedManifest(raw) {
		t.Fatalf("installed native manifest is not owned: %v %q", err, raw)
	}
	if err := removeAt(target); err != nil || registered != "" {
		t.Fatalf("Browser Entry removal registration = %q, error = %v", registered, err)
	}
	if _, err := os.Lstat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("owned native manifest survived removal: %v", err)
	}
}

func TestInstallRefusesToReplaceAForeignNativeManifest(t *testing.T) {
	root := t.TempDir()
	host := writeInstallerFile(t, root, "ardents-browser-entry", []byte("host"))
	extension := writeExtension(t, root)
	manifestPath := writeInstallerFile(t, root, "manifest.json", []byte(`{"name":"foreign"}`))
	registered := false
	target := location{path: manifestPath, register: func(string) error { registered = true; return nil }, unregister: func(string) error { return nil }}
	if _, err := installAt(host, extension, target); err == nil || registered {
		t.Fatalf("foreign Browser Entry manifest installation error = %v, registered = %t", err, registered)
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil || string(raw) != `{"name":"foreign"}` {
		t.Fatalf("foreign manifest was changed: %v %q", err, raw)
	}
}

func TestInstallFailsClosedWithoutASelectedPlatformLocation(t *testing.T) {
	root := t.TempDir()
	host := writeInstallerFile(t, root, "ardents-browser-entry", []byte("host"))
	extension := writeExtension(t, root)
	if _, err := installAt(host, extension, location{}); err == nil {
		t.Fatal("Browser Entry installation accepted an unsupported platform location")
	}
}

func TestInstallRejectsAnXPIWithoutMozillaCOSESignatureSurface(t *testing.T) {
	root := t.TempDir()
	host := writeInstallerFile(t, root, "ardents-browser-entry", []byte("host"))
	unsigned := writeExtensionArchive(t, root, false)
	target := location{path: filepath.Join(root, "native", "manifest.json"), register: func(string) error { return nil }, unregister: func(string) error { return nil }}
	if _, err := installAt(host, unsigned, target); err == nil {
		t.Fatal("Browser Entry installation accepted an XPI without Mozilla COSE metadata")
	}
}

func TestInstallDoesNotReuseARecoverableFixedTemporaryName(t *testing.T) {
	root := t.TempDir()
	host := writeInstallerFile(t, root, "ardents-browser-entry", []byte("host"))
	extension := writeExtension(t, root)
	manifestPath := filepath.Join(root, "native", "manifest.json")
	legacyTemporary := manifestPath + ".next"
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyTemporary, []byte("foreign stale temporary"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := location{path: manifestPath, register: func(string) error { return nil }, unregister: func(string) error { return nil }}
	if _, err := installAt(host, extension, target); err != nil {
		t.Fatal(err)
	}
	stale, err := os.ReadFile(legacyTemporary)
	if err != nil || string(stale) != "foreign stale temporary" {
		t.Fatalf("fixed temporary was changed: %v %q", err, stale)
	}
}

func writeExtension(t *testing.T, root string) string {
	t.Helper()
	return writeExtensionArchive(t, root, true)
}

func writeExtensionArchive(t *testing.T, root string, signed bool) string {
	t.Helper()
	path := filepath.Join(root, "ardents-alpha-browser-entry.xpi")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(map[string]any{"browser_specific_settings": map[string]any{"gecko": map[string]string{"id": browserentry.FirefoxExtensionID}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if signed {
		for _, name := range []string{"META-INF/cose.manifest", "META-INF/cose.sig"} {
			metadata, metadataErr := archive.Create(name)
			if metadataErr != nil {
				t.Fatal(metadataErr)
			}
			if _, metadataErr := metadata.Write([]byte("Mozilla signature metadata")); metadataErr != nil {
				t.Fatal(metadataErr)
			}
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

func writeInstallerFile(t *testing.T, root, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

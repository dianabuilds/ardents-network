package replacement

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLoadBundleBuildsOnlyByteReleaseInput(t *testing.T) {
	root := t.TempDir()
	descriptor := "schema=ardents-offline-replacement-bundle-v1\n" +
		"target_path=ardents/" + runtime.GOOS + "-" + runtime.GOARCH + "/ardents\n" +
		"artifact=ardents\ntrusted_root=root.json\nplatform=" + runtime.GOOS + "-" + runtime.GOARCH +
		"\narchitecture=" + runtime.GOARCH + "\nenvironment=h4-alpha\nnetwork=ardents-alpha\n"
	for name, contents := range map[string]string{"REPLACEMENT": descriptor, "ardents": "candidate", "root.json": "root", "timestamp.json": "metadata"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	inputs, err := LoadBundle(root, time.Unix(5, 0))
	if err != nil {
		t.Fatalf("LoadBundle() error = %v", err)
	}
	if string(inputs.Artifact) != "candidate" || string(inputs.RootBytes) != "root" ||
		string(inputs.Files["https://release.invalid/metadata/timestamp.json"]) != "metadata" {
		t.Fatalf("LoadBundle() inputs = %#v", inputs)
	}
}

func TestLoadBundleRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "unexpected"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundle(root, time.Unix(5, 0)); err == nil {
		t.Fatal("LoadBundle() succeeded with a directory entry")
	}
}

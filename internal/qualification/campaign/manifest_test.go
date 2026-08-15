package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishManifestIsImmutable(t *testing.T) {
	root := t.TempDir()
	raw := []byte(`{"schema":"test"}`)
	if err := PublishManifest(root, raw); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(root, "campaign-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(raw) {
		t.Fatalf("stored manifest = %q", stored)
	}
	if err := PublishManifest(root, raw); err != nil {
		t.Fatalf("identical manifest was not idempotent: %v", err)
	}
	if err := PublishManifest(root, []byte(`{"schema":"changed"}`)); err == nil {
		t.Fatal("immutable campaign manifest was replaced")
	}
}

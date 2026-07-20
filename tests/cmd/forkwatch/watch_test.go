package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractManifestValue(t *testing.T) {
	got := extractManifestValue("- `Pinned upstream baseline`: `v0.10.1`")
	if got != "v0.10.1" {
		t.Fatalf("baseline = %q, want v0.10.1", got)
	}
}

func TestParseManifestRejectsMissingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FORK.md")
	if err := os.WriteFile(path, []byte("# bad manifest\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if _, err := parseManifest(path); err == nil {
		t.Fatal("expected parseManifest to fail for incomplete manifest")
	}
}

func TestParseManifestReadsRequiredFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FORK.md")
	body := `- ` + "`Module`" + `: ` + "`github.com/waku-org/go-waku`" + `
- ` + "`Upstream source`" + `: ` + "`https://github.com/waku-org/go-waku`" + `
- ` + "`Pinned upstream baseline`" + `: ` + "`v0.10.1`" + `
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	manifest, err := parseManifest(path)
	if err != nil {
		t.Fatalf("parseManifest error: %v", err)
	}
	if manifest.Module != "github.com/waku-org/go-waku" {
		t.Fatalf("module = %q", manifest.Module)
	}
	if manifest.UpstreamSource != "https://github.com/waku-org/go-waku" {
		t.Fatalf("source = %q", manifest.UpstreamSource)
	}
	if manifest.PinnedBaseline != "v0.10.1" {
		t.Fatalf("baseline = %q", manifest.PinnedBaseline)
	}
}

package browserentry

import (
	"path/filepath"
	"testing"
)

func TestDefaultStatePathIsAbsoluteAndUsesTheReleasedFilename(t *testing.T) {
	path, err := DefaultStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("default Browser Entry state path is not absolute: %q", path)
	}
	if filepath.Base(path) != "alpha-proxy.json" {
		t.Fatalf("default Browser Entry state filename = %q", filepath.Base(path))
	}
}

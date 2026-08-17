package modulecache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalArchiveIsStable(t *testing.T) {
	root := fixtureCache(t)
	first := filepath.Join(t.TempDir(), "first.tar.gz")
	second := filepath.Join(t.TempDir(), "second.tar.gz")
	if err := publishCanonicalArchive(root, first); err != nil {
		t.Fatal(err)
	}
	if err := publishCanonicalArchive(root, second); err != nil {
		t.Fatal(err)
	}
	firstHash, firstSize, firstErr := hashFile(first)
	secondHash, secondSize, secondErr := hashFile(second)
	if firstErr != nil || secondErr != nil || firstHash != secondHash || firstSize != secondSize {
		t.Fatalf("canonical archives differ: %s/%d %s/%d errors=%v/%v",
			firstHash, firstSize, secondHash, secondSize, firstErr, secondErr)
	}
}

func TestFailedArchiveLeavesNoPublishedOrPartialFile(t *testing.T) {
	root := fixtureCache(t)
	if err := os.Symlink("missing", filepath.Join(root, "unsupported-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	parent := t.TempDir()
	target := filepath.Join(parent, "gomodcache.tar.gz")
	if err := publishCanonicalArchive(root, target); err == nil {
		t.Fatal("unsupported cache entry was accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed archive left files=%v err=%v", entries, err)
	}
}

func fixtureCache(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "example@v1.0.0")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "module.go"), []byte("package example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

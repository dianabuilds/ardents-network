package nativecircuit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNativeFixtureConfigsAreReadableWithoutDACOverride(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not preserve Unix permission bits for Linux bind mounts")
	}
	fixture, err := prepareNativeFixture(t.TempDir(), "permissions", "", &nativeWorkload{
		Profile: workloadDirect, Kind: "setup", Seed: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"user", "service"} {
		assertReadableFixtureDirectory(t, filepath.Join(fixture.root, "configs", role))
	}
	for _, role := range []string{"shape-user", "capture-user"} {
		assertReadableFixtureDirectory(t, filepath.Join(fixture.root, "tool-configs", role))
	}
}

func assertReadableFixtureDirectory(t *testing.T, directory string) {
	t.Helper()
	assertFixtureMode(t, directory, 0o755)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		assertFixtureMode(t, filepath.Join(directory, entry.Name()), 0o644)
	}
}

func assertFixtureMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual := info.Mode().Perm(); actual != expected {
		t.Fatalf("%s mode = %o, want %o", filepath.Base(path), actual, expected)
	}
}

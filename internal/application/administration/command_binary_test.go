package administration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildArdentsNext builds the real CLI for Administration boundary checks.
func buildArdentsNext(t *testing.T) string {
	t.Helper()
	prebuilt := os.Getenv("ARDENTS_E2E_COMMAND")
	if prebuilt == "" {
		prebuilt = os.Getenv("ARDENTS_PREBUILT_NEXT")
	}
	if prebuilt != "" {
		info, err := os.Stat(prebuilt)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("prebuilt Ardents Next command is not a regular file: %v", err)
		}
		return prebuilt
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E source")
	}
	name := "ardents-next"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "./cmd/ardents-next")
	build.Dir = filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build Ardents Next command: %v\n%s", err, output)
	}
	return path
}

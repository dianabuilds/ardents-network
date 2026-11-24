package source_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func buildSourceCommand(t *testing.T) string {
	t.Helper()
	name := "ardents"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", binary, "./cmd/ardents-next")
	build.Dir = filepath.Join("..", "..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}
	return binary
}

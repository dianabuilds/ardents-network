package node_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func buildNodeCommand(t *testing.T) string {
	t.Helper()
	filename := "ardents"
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	binary := filepath.Join(t.TempDir(), filename)
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", binary, "./cmd/ardents-next")
	build.Dir = filepath.Join("..", "..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}
	return binary
}

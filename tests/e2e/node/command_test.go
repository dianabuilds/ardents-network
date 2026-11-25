package node_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// Every scenario owns its own mutable roots. The compiled command itself is
// immutable and can be built once for the process-test package.
var nodeCommandBuild struct {
	once      sync.Once
	directory string
	path      string
	output    []byte
	err       error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if nodeCommandBuild.directory != "" {
		if err := os.RemoveAll(nodeCommandBuild.directory); err != nil {
			fmt.Fprintln(os.Stderr, "test binary cleanup failed:", err)
			code = 1
		}
	}
	os.Exit(code)
}

func buildNodeCommand(t *testing.T) string {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_PREBUILT_NEXT"); prebuilt != "" {
		return prebuilt
	}
	nodeCommandBuild.once.Do(func() {
		nodeCommandBuild.directory, nodeCommandBuild.err = os.MkdirTemp("", "ardents-next-e2e-")
		if nodeCommandBuild.err != nil {
			return
		}
		filename := "ardents"
		if runtime.GOOS == "windows" {
			filename += ".exe"
		}
		nodeCommandBuild.path = filepath.Join(nodeCommandBuild.directory, filename)
		build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", nodeCommandBuild.path, "./cmd/ardents-next")
		build.Dir = filepath.Join("..", "..", "..")
		nodeCommandBuild.output, nodeCommandBuild.err = build.CombinedOutput()
	})
	if nodeCommandBuild.err != nil {
		t.Fatalf("build CLI: %v: %s", nodeCommandBuild.err, nodeCommandBuild.output)
	}
	return nodeCommandBuild.path
}
